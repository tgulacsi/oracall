// Copyright 2017, 2022 Tamas Gulacsi
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
	"time"

	"github.com/go-logr/logr"
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

var Timeout = DefaultTimeout

const DefaultTimeout = time.Hour

var bufpool = bp.New(4096)

func GRPCServer(globalCtx context.Context, logger logr.Logger, verbose bool, checkAuth func(ctx context.Context, path string) error, options ...grpc.ServerOption) *grpc.Server {
	erroredMethods := make(map[string]struct{})
	var erroredMethodsMu sync.RWMutex

	getLogger := func(ctx context.Context, fullMethod string) (logr.Logger, func(error), context.Context, context.CancelFunc) {
		var cancel context.CancelFunc = func() {}
		if Timeout != 0 {
			ctx, cancel = context.WithTimeout(ctx, Timeout) //nolint:govet
		}
		reqID := ContextGetReqID(ctx)
		ctx = ContextWithReqID(ctx, reqID)
		lgr := logger.WithValues("reqID", reqID)
		ctx = logr.NewContext(ctx, lgr)
		verbose := verbose
		var wasThere bool
		if !verbose {
			erroredMethodsMu.RLock()
			_, verbose = erroredMethods[fullMethod]
			erroredMethodsMu.RUnlock()
			wasThere = verbose
		} else {
			logger := logger.WithName("godror")
			godror.SetLogger(logger)
			ctx = logr.NewContext(ctx, logger)
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
				if false {
					defer func() {
						if r := recover(); r != nil {
							trace := stack.Trace().String()
							var ok bool
							if err, ok = r.(error); ok {
								logger.Error(err, "PANIC", "trace", trace)
								return
							}
							err = fmt.Errorf("%+v", r)
							logger.Error(err, "PANIC", "trace", trace)
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
				lgr.Info("handler", "RESP", info.FullMethod, "dur", time.Since(start).String(), "error", err)
				commit(err)
				return StatusError(err)
			}),

		grpc.UnaryInterceptor(
			func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
				if false {
					defer func() {
						if r := recover(); r != nil {
							trace := stack.Trace().String()
							var ok bool
							if err, ok = r.(error); ok {
								logger.Error(err, "PANIC", "trace", trace)
								return
							}
							err = fmt.Errorf("%+v", r)
							logger.Error(err, "PANIC", "trace", trace)
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
					logger.Error(err, "marshal", "req", req)
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

				logger.Info("handled", "RESP", info.FullMethod, "dur", time.Since(start).String(), "error", err)
				commit(err)

				buf.Reset()
				if jErr := jenc.Encode(res); err != nil {
					logger.Error(err, "marshal", jErr, "res", res)
				}
				logger.Info("encoded", "RESP", res, "error", err)

				return res, StatusError(err)
			}),
	}
	return grpc.NewServer(append(opts, options...)...)
}

func StatusError(err error) error {
	if err == nil {
		return err
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
