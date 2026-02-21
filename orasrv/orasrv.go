// Copyright 2017, 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package orasrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/UNO-SOFT/w3ctrace"
	"github.com/UNO-SOFT/w3ctrace/gtrace"
	"github.com/UNO-SOFT/zlog/v2"
	"github.com/UNO-SOFT/zlog/v2/slog"

	"github.com/tgulacsi/go/iohlp"
	oracall "github.com/tgulacsi/oracall/lib"

	"github.com/go-stack/stack"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	godror "github.com/godror/godror"
)

//go:generate buf generate

var Timeout = DefaultTimeout

const (
	DefaultTimeout = time.Hour

	catchPanic = false
)

func FromContext(ctx context.Context) *slog.Logger { return oracall.FromContext(ctx) }
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return oracall.WithContext(ctx, logger)
}
func NewT(t *testing.T) *slog.Logger { return zlog.NewT(t).SLog() }

func GRPCServer(globalCtx context.Context, logger *slog.Logger, verbose bool, checkAuth func(ctx context.Context, path string) error, options ...grpc.ServerOption) *grpc.Server {
	erroredMethods := make(map[string]struct{})
	var erroredMethodsMu sync.RWMutex

	getLogger := func(ctx context.Context, fullMethod string) (*slog.Logger, func(error), context.Context, context.CancelFunc) {
		var cancel context.CancelFunc = func() {}
		if Timeout != 0 {
			ctx, cancel = context.WithTimeout(ctx, Timeout) //nolint:govet
		}
		if tr := gtrace.FromIncomingContext(ctx); tr.IsValid() {
			ctx = w3ctrace.NewContext(ctx, tr)
		}
		reqID := ContextGetReqID(ctx)
		ctx = ContextWithReqID(ctx, reqID)
		lgr := logger.With("reqID", reqID)
		ctx = zlog.NewSContext(ctx, lgr)
		verbose := verbose
		var wasThere bool
		if !verbose {
			erroredMethodsMu.RLock()
			_, verbose = erroredMethods[fullMethod]
			erroredMethodsMu.RUnlock()
			wasThere = verbose
		} else {
			godror.SetLogger(logger.WithGroup("godror"))
			ctx = zlog.NewSContext(ctx, logger)
		}
		commit := func(err error) {
			if wasThere && err == nil {
				erroredMethodsMu.Lock()
				delete(erroredMethods, fullMethod)
				erroredMethodsMu.Unlock()
			} else if err != nil && !wasThere {
				erroredMethodsMu.Lock()
				erroredMethods[fullMethod] = struct{}{}
				erroredMethodsMu.Unlock()
			}
		}
		return lgr, commit, ctx, cancel
	}

	var catchPanicF func()
	if catchPanic {
		catchPanicF = func() {
			if r := recover(); r != nil {
				trace := stack.Trace().String()
				if err, ok := r.(error); ok {
					logger.Error("PANIC", "trace", trace, "error", err)
					return
				}
				logger.Error("PANIC", "trace", trace, "error", fmt.Errorf("%+v", r))
			}
		}
	}

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(
			func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
				if catchPanic {
					defer catchPanicF()
				}
				lgr, commit, ctx, cancel := getLogger(ss.Context(), info.FullMethod)
				defer cancel()

				lgr = lgr.With("method", info.FullMethod)
				lgr.Info("checkAuth")
				if err = checkAuth(ctx, info.FullMethod); err != nil {
					return status.Error(codes.Unauthenticated, err.Error())
				}

				wss := grpc_middleware.WrapServerStream(ss)
				wss.WrappedContext = ctx
				start := time.Now()
				err = handler(srv, wss)
				dur := time.Since(start)
				commit(err)
				lvl := slog.LevelInfo
				if err != nil {
					lvl = slog.LevelError
				}
				lgr.Log(ctx, lvl, "handler", "dur", dur.String(), "error", err)
				return StatusError(err)
			}),

		grpc.UnaryInterceptor(
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, err error) {
				if catchPanic {
					defer catchPanicF()
				}
				logger, commit, ctx, cancel := getLogger(ctx, info.FullMethod)
				defer cancel()
				logger = logger.With("method", info.FullMethod)

				if err = checkAuth(ctx, info.FullMethod); err != nil {
					return nil, status.Error(codes.Unauthenticated, err.Error())
				}

				ht := &iohlp.HeadTailKeeper{Limit: 1024}
				jenc := json.NewEncoder(ht)
				if err = jenc.Encode(req); err != nil {
					logger.Error("marshal", "req", req, "error", err)
				}
				logger = logger.With("request", ht.String())
				if logger.Enabled(ctx, slog.LevelDebug) {
					logger.Debug("marshaled")
				}

				// Fill PArgsHidden
				if r := reflect.ValueOf(req).Elem(); r.Kind() != reflect.Struct {
					logger.Info("not struct", "req", fmt.Sprintf("%T %#v", req, req))
				} else {
					if f := r.FieldByName("PArgsHidden"); f.IsValid() {
						f.Set(reflect.ValueOf(ht.String()))
					}
				}

				start := time.Now()
				res, err := handler(ctx, req)
				dur := time.Since(start)
				commit(err)

				ht.Reset()
				if jErr := jenc.Encode(res); jErr != nil {
					fmt.Fprintf(ht, ": %+v", res)
					logger.Error("marshal", "response", ht.String(), "error", jErr)
				}
				lvl := slog.LevelInfo
				if err != nil {
					lvl = slog.LevelError
				}
				logger.Log(ctx, lvl, "handled", "response", ht.String(),
					"dur", dur.String(), "error", err)

				return res, StatusError(err)
			}),
	}
	// it should be implemented in checkAuth
	// nosemgrep: go.grpc.security.grpc-server-insecure-connection.grpc-server-insecure-connection
	return grpc.NewServer(append(opts, options...)...)
}

func StatusError(err error) error {
	if err == nil {
		return nil
	}
	var code codes.Code
	var sc interface {
		Code() codes.Code
	}
	if errors.Is(err, oracall.ErrInvalidArgument) {
		code = codes.InvalidArgument
	} else if errors.As(err, &sc) && sc != nil {
		code = sc.Code()
	}
	if code == 0 {
		return err
	}
	return status.New(code, err.Error()).Err()
}

type reqIDCtxKey struct{}

func ContextWithReqID(ctx context.Context, reqID string) context.Context {
	if reqID == "" {
		if tr := w3ctrace.FromContext(ctx); tr.IsValid() {
			reqID = tr.ShortString()
		} else {
			reqID = NewULID()
		}
	}
	return context.WithValue(ctx, reqIDCtxKey{}, reqID)
}
func ContextGetReqID(ctx context.Context) string {
	if reqID, ok := ctx.Value(reqIDCtxKey{}).(string); ok {
		return reqID
	} else if tr := w3ctrace.FromContext(ctx); tr.IsValid() {
		return tr.ShortString()
	}
	return NewULID()
}
func NewULID() string {
	return ulid.MustNew(ulid.Now(), ulid.DefaultEntropy()).String()
}
