package orasrv

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/gogo/protobuf/proto"
	bp "github.com/tgulacsi/go/bufpool"
	oracall "github.com/tgulacsi/oracall/lib"
	errors "golang.org/x/xerrors"

	"github.com/go-kit/kit/log"
	"github.com/oklog/ulid"

	"github.com/go-stack/stack"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	goracle "gopkg.in/goracle.v2"
)

var Timeout = DefaultTimeout

const DefaultTimeout = time.Hour

var bufpool = bp.New(4096)

func GRPCServer(globalCtx context.Context, logger log.Logger, verbose bool, checkAuth func(ctx context.Context, path string) error, options ...grpc.ServerOption) *grpc.Server {
	erroredMethods := make(map[string]struct{})
	var erroredMethodsMu sync.RWMutex

	getLogger := func(ctx context.Context, fullMethod string) (log.Logger, func(error), context.Context, context.CancelFunc) {
		var toCancel context.CancelFunc
		if _, ok := ctx.Deadline(); !ok && Timeout > 0 {
			ctx, toCancel = context.WithTimeout(ctx, Timeout) //nolint:govet
		}
		var cancel context.CancelFunc
		ctx, cancel = joincontext.Join(ctx, globalCtx)
		if toCancel != nil {
			origCancel := cancel
			cancel = func() { toCancel(); origCancel() }
		}
		reqID := ContextGetReqID(ctx)
		ctx = ContextWithReqID(ctx, reqID)
		lgr := log.With(logger, "reqID", reqID)
		ctx = ContextWithLogger(ctx, lgr)
		verbose := verbose
		var wasThere bool
		if !verbose {
			erroredMethodsMu.RLock()
			_, verbose = erroredMethods[fullMethod]
			erroredMethodsMu.RUnlock()
			wasThere = verbose
		}
		if verbose {
			ctx = goracle.ContextWithLog(ctx, log.With(lgr, "lib", "goracle").Log)
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
				defer func() {
					if r := recover(); r != nil {
						trace := stack.Trace().String()
						var ok bool
						if err, ok = r.(error); ok {
							logger.Log("PANIC", err, "trace", trace)
							return
						}
						err = errors.Errorf("%+v", r)
						logger.Log("PANIC", fmt.Sprintf("%+v", err), "trace", trace)
					}
				}()
				lgr, commit, ctx, cancel := getLogger(ss.Context(), info.FullMethod)
				defer cancel()

				buf := bufpool.Get()
				defer bufpool.Put(buf)
				jenc := json.NewEncoder(buf)
				if err = jenc.Encode(srv); err != nil {
					lgr.Log("marshal error", err, "srv", srv)
				}
				lgr.Log("REQ", info.FullMethod, "srv", buf.String())
				if err = checkAuth(ctx, info.FullMethod); err != nil {
					return status.Error(codes.Unauthenticated, err.Error())
				}

				wss := grpc_middleware.WrapServerStream(ss)
				wss.WrappedContext = ctx
				err = handler(srv, wss)

				lgr.Log("RESP", info.FullMethod, "error", err)
				commit(err)
				return StatusError(err)
			}),

		grpc.UnaryInterceptor(
			func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
				defer func() {
					if r := recover(); r != nil {
						trace := stack.Trace().String()
						var ok bool
						if err, ok = r.(error); ok {
							logger.Log("PANIC", err, "trace", trace)
							return
						}
						err = errors.Errorf("%+v", r)
						logger.Log("PANIC", fmt.Sprintf("%+v", err), "trace", trace)
					}
				}()
				logger, commit, ctx, cancel := getLogger(ctx, info.FullMethod)
				defer cancel()

				if err = checkAuth(ctx, info.FullMethod); err != nil {
					return nil, status.Error(codes.Unauthenticated, err.Error())
				}

				buf := bufpool.Get()
				defer bufpool.Put(buf)
				jenc := json.NewEncoder(buf)
				if err = jenc.Encode(req); err != nil {
					logger.Log("marshal error", err, "req", req)
				}
				logger.Log("REQ", info.FullMethod, "req", buf.String())

				// Fill PArgsHidden
				if r := reflect.ValueOf(req).Elem(); r.Kind() != reflect.Struct {
					logger.Log("error", "not struct", "req", fmt.Sprintf("%T %#v", req, req))
				} else {
					if f := r.FieldByName("PArgsHidden"); f.IsValid() {
						f.Set(reflect.ValueOf(buf.String()))
					}
				}

				res, err := handler(ctx, req)

				logger.Log("RESP", info.FullMethod, "error", err)
				commit(err)

				buf.Reset()
				if jErr := jenc.Encode(res); err != nil {
					logger.Log("marshal error", jErr, "res", res)
				}
				logger.Log("RESP", res, "error", err)

				return res, StatusError(err)
			}),
	}
	return grpc.NewServer(opts...)
}

func StatusError(err error) error {
	if err == nil {
		return err
	}
	var code codes.Code
	if errors.Is(err, oracall.ErrInvalidArgument) {
		code = codes.InvalidArgument
	} else if sc, ok := errors.Unwrap(err).(interface {
		Code() codes.Code
	}); ok {
		code = sc.Code()
	}
	if code == 0 {
		return err
	}
	s := status.New(code, err.Error())
	if sd, sErr := s.WithDetails(&pbMessage{Message: fmt.Sprintf("%+v", err)}); sErr == nil {
		s = sd
	}
	return s.Err()
}

type pbMessage struct {
	Message string
}

func (m pbMessage) ProtoMessage()   {}
func (m *pbMessage) Reset()         { m.Message = "" }
func (m *pbMessage) String() string { return proto.MarshalTextString(m) }

type ctxKey string

const reqIDCtxKey = ctxKey("reqID")
const loggerCtxKey = ctxKey("logger")

func ContextWithLogger(ctx context.Context, logger log.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}
func ContextGetLogger(ctx context.Context) log.Logger {
	if lgr, ok := ctx.Value(loggerCtxKey).(log.Logger); ok {
		return lgr
	}
	return nil
}
func ContextWithReqID(ctx context.Context, reqID string) context.Context {
	if reqID == "" {
		reqID = NewULID()
	}
	return context.WithValue(ctx, reqIDCtxKey, reqID)
}
func ContextGetReqID(ctx context.Context) string {
	if reqID, ok := ctx.Value(reqIDCtxKey).(string); ok {
		return reqID
	}
	return NewULID()
}
func NewULID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
