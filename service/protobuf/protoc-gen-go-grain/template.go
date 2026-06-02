package main

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/asynkron/protoactor-go/service/protobuf/protoc-gen-go-grain/options"
)

//go:embed templates/grain.tmpl
var grainTemplate string

//go:embed templates/grain_client.tmpl
var grainClientTemplate string

//go:embed templates/grain_client_init.tmpl
var grainClientInitTemplate string

//go:embed templates/actor_client.tmpl
var actorClientTemplate string

//go:embed templates/actor.tmpl
var actorTemplate string

//go:embed templates/error.tmpl
var errorTemplate string

type serviceDesc struct {
	Name                  string // Greeter
	ClusterImportPath     string
	ClusterImportPathName string
	UsePlacementContext   bool
	Kind                  string
	NodeType              string
	Actor                 string
	RouteKeyField         string
	UseGrainactor         bool
	Methods               []*methodDesc
}

type methodDesc struct {
	Name    string
	Input   string
	Output  string
	Index   int
	Options *options.MethodOptions
}

type actorDesc struct {
	Name          string
	Actor         string
	Kind          string
	NodeType      string
	RouteKeyField string
	Services      []*serviceDesc
	Methods       []*actorMethodDesc
}

type actorMethodDesc struct {
	Service *serviceDesc
	Method  *methodDesc
	Index   int
}

type errorDesc struct {
	Name       string
	Value      string
	CamelValue string
	Comment    string
	HasComment bool
}

type errorsWrapper struct {
	Errors []*errorDesc
}

type grainClientInitDesc struct {
	Services []string
}

func (es *errorsWrapper) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("error").Parse(strings.TrimSpace(errorTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, es); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (s *serviceDesc) execute() string {
	return strings.Trim(s.executeClient()+"\n\n"+s.executeServer(), "\r\n")
}

func (s *serviceDesc) executeClient() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("grain_client").Funcs(template.FuncMap{
		"lowerFirst": lowerFirst,
		"toCamel":    toCamel,
	}).Parse(strings.TrimSpace(grainClientTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, s); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (s *serviceDesc) executeServer() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("grain").Funcs(template.FuncMap{
		"lowerFirst": lowerFirst,
		"toCamel":    toCamel,
	}).Parse(strings.TrimSpace(grainTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, s); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (a *actorDesc) executeClient() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("actor_client").Funcs(template.FuncMap{
		"lowerFirst": lowerFirst,
		"toCamel":    toCamel,
	}).Parse(strings.TrimSpace(actorClientTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, a); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (a *actorDesc) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("actor").Funcs(template.FuncMap{
		"lowerFirst": lowerFirst,
		"toCamel":    toCamel,
	}).Parse(strings.TrimSpace(actorTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, a); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (d *grainClientInitDesc) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("grain_client_init").Funcs(template.FuncMap{
		"lowerFirst": lowerFirst,
	}).Parse(strings.TrimSpace(grainClientInitTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, d); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
