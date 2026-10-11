package grpcrunner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Source provides service descriptors, either from server reflection or
// from local .proto files.
type Source interface {
	ListServices(ctx context.Context) ([]string, error)
	FindService(ctx context.Context, name string) (protoreflect.ServiceDescriptor, error)
	Close()
}

type reflectionSource struct {
	client *grpcreflect.Client
}

// NewReflectionSource uses the server reflection API (v1 with v1alpha
// fallback).
func NewReflectionSource(ctx context.Context, conn grpc.ClientConnInterface) Source {
	return &reflectionSource{client: grpcreflect.NewClientAuto(ctx, conn)}
}

func (r *reflectionSource) ListServices(context.Context) ([]string, error) {
	names, err := r.client.ListServices()
	if err != nil {
		return nil, fmt.Errorf("server reflection: %w", err)
	}
	var out []string
	for _, n := range names {
		if !strings.HasPrefix(n, "grpc.reflection.") {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *reflectionSource) FindService(_ context.Context, name string) (protoreflect.ServiceDescriptor, error) {
	sd, err := r.client.ResolveService(name)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", name, err)
	}
	return sd.UnwrapService(), nil
}

func (r *reflectionSource) Close() { r.client.Reset() }

type fileSource struct {
	services map[string]protoreflect.ServiceDescriptor
}

// NewFileSource compiles .proto files. Absolute file paths outside every
// import path get their directory added as an import path.
func NewFileSource(importPaths, files []string) (Source, error) {
	if len(files) == 0 {
		return nil, errors.New("no .proto files given")
	}
	paths := append([]string(nil), importPaths...)
	names := make([]string, 0, len(files))
	for _, f := range files {
		name := f
		if filepath.IsAbs(f) {
			name = ""
			for _, ip := range paths {
				if rel, err := filepath.Rel(ip, f); err == nil && !strings.HasPrefix(rel, "..") {
					name = filepath.ToSlash(rel)
					break
				}
			}
			if name == "" {
				paths = append(paths, filepath.Dir(f))
				name = filepath.Base(f)
			}
		}
		names = append(names, name)
	}
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: paths}),
	}
	compiled, err := c.Compile(context.Background(), names...)
	if err != nil {
		return nil, err
	}
	fs := &fileSource{services: map[string]protoreflect.ServiceDescriptor{}}
	for _, fd := range compiled {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			fs.services[string(svcs.Get(i).FullName())] = svcs.Get(i)
		}
	}
	return fs, nil
}

func (f *fileSource) ListServices(context.Context) ([]string, error) {
	out := make([]string, 0, len(f.services))
	for n := range f.services {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fileSource) FindService(_ context.Context, name string) (protoreflect.ServiceDescriptor, error) {
	if sd, ok := f.services[name]; ok {
		return sd, nil
	}
	return nil, fmt.Errorf("service %s not found in proto files", name)
}

func (f *fileSource) Close() {}

type MethodInfo struct {
	Name            string `json:"name"`
	FullName        string `json:"fullName"`
	InputType       string `json:"inputType"`
	OutputType      string `json:"outputType"`
	ClientStreaming bool   `json:"clientStreaming"`
	ServerStreaming bool   `json:"serverStreaming"`
	Template        string `json:"template"`
}

type ServiceInfo struct {
	Name    string       `json:"name"`
	Methods []MethodInfo `json:"methods"`
}

// Describe lists every service and method (sorted) with a JSON template for
// each method's input message.
func Describe(ctx context.Context, src Source) ([]ServiceInfo, error) {
	names, err := src.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []ServiceInfo
	for _, n := range names {
		sd, err := src.FindService(ctx, n)
		if err != nil {
			return nil, err
		}
		si := ServiceInfo{Name: n}
		ms := sd.Methods()
		for i := 0; i < ms.Len(); i++ {
			m := ms.Get(i)
			si.Methods = append(si.Methods, MethodInfo{
				Name:            string(m.Name()),
				FullName:        string(m.FullName()),
				InputType:       string(m.Input().FullName()),
				OutputType:      string(m.Output().FullName()),
				ClientStreaming: m.IsStreamingClient(),
				ServerStreaming: m.IsStreamingServer(),
				Template:        Template(m.Input()),
			})
		}
		sort.Slice(si.Methods, func(i, j int) bool { return si.Methods[i].Name < si.Methods[j].Name })
		out = append(out, si)
	}
	return out, nil
}
