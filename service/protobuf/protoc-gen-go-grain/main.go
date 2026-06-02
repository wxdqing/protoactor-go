package main

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	options := protogen.Options{
		ParamFunc: func(name string, value string) error {
			switch name {
			case "cluster_import":
				return setClusterImportPath(value)
			default:
				return fmt.Errorf("unknown parameter %q", name)
			}
		},
	}

	options.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f)
		}
		generateGrainClientInitFile(gen)

		return nil
	})
}
