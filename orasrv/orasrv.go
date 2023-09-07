// Copyright 2017, 2023 Tamas Gulacsi
//
// SPDX-License-Identifier: Apache-2.0

package orasrv

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/UNO-SOFT/zlog/v2/slog"

	bp "github.com/tgulacsi/go/bufpool"
	oracall "github.com/tgulacsi/oracall/lib"

	"github.com/oklog/ulid"

	"github.com/go-stack/stack"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	godror "github.com/godror/godror"
)

var (
	Timeout = DefaultTimeout

	bufpool = bp.New(4096)
)

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

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(
			func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
				if catchPanic {
					defer func() {
						if r := recover(); r != nil {
							trace := stack.Trace().String()
							var ok bool
							if err, ok = r.(error); ok {
								logger.Error("PANIC", "trace", trace, "error", err)
								return
							}
							err = fmt.Errorf("%+v", r)
							logger.Error("PANIC", "trace", trace, "error", err)
						}
					}()
				}
				lgr, commit, ctx, cancel := getLogger(ss.Context(), info.FullMethod)
				defer cancel()

				lgr.Info("checkAuth", "REQ", info.FullMethod)
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
				lgr.Log(ctx, lvl, "handler", "method", info.FullMethod, "dur", dur.String(), "error", err)
				return StatusError(err)
			}),

		grpc.UnaryInterceptor(
			func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
				if catchPanic {
					defer func() {
						if r := recover(); r != nil {
							trace := stack.Trace().String()
							var ok bool
							if err, ok = r.(error); ok {
								logger.Error("PANIC", "trace", trace, "error", err)
								return
							}
							err = fmt.Errorf("%+v", r)
							logger.Error("PANIC", "trace", trace, "error", err)
						}
					}()
				}
				logger, commit, ctx, cancel := getLogger(ctx, info.FullMethod)
				defer cancel()

				if err = checkAuth(ctx, info.FullMethod); err != nil {
					return nil, status.Error(codes.Unauthenticated, err.Error())
				}

				buf := bufpool.Get()
				defer bufpool.Put(buf)
				jenc := json.NewEncoder(buf)
				if err = jenc.Encode(req); err != nil {
					logger.Error("marshal", "req", req, "error", err)
				}
				logger.Info("marshaled", "REQ", info.FullMethod, "req", buf.String())

				// Fill PArgsHidden
				if r := reflect.ValueOf(req).Elem(); r.Kind() != reflect.Struct {
					logger.Info("not struct", "req", fmt.Sprintf("%T %#v", req, req))
				} else {
					if f := r.FieldByName("PArgsHidden"); f.IsValid() {
						f.Set(reflect.ValueOf(buf.String()))
					}
				}

				start := time.Now()
				res, err := handler(ctx, req)
				dur := time.Since(start)
				commit(err)

				buf.Reset()
				if jErr := jenc.Encode(res); jErr != nil {
					buf.Reset()
					fmt.Fprintf(buf, "%+v", res)
					logger.Error("marshal", "response", buf.String(), "error", jErr)
				}
				lvl := slog.LevelInfo
				if err != nil {
					lvl = slog.LevelError
				}
				logger.Log(ctx, lvl, "handled", "method", info.FullMethod, "response", buf.String(),
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
	} else if errors.As(err, &sc) {
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
		reqID = NewULID()
	}
	return context.WithValue(ctx, reqIDCtxKey{}, reqID)
}
func ContextGetReqID(ctx context.Context) string {
	if reqID, ok := ctx.Value(reqIDCtxKey{}).(string); ok {
		return reqID
	}
	return NewULID()
}
func NewULID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
