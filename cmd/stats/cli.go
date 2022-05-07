package stats

import (
	"fmt"
	"strings"

	"github.com/mikhailshilkov/schema-tools/pkg"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "stats",
		Short: "Get the stats of a current schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			schemaUrl := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/master/provider/cmd/pulumi-resource-%[1]s/schema.json", provider)
			sch := pkg.DownloadSchema(schemaUrl)

			//usr, _ := user.Current()
			//basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)

			//path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s/schema.json", provider)
			//schemaPath := filepath.Join(basePath, path)
			//sch := readSchema(schemaPath)

			uniques := codegen.NewStringSet()
			visitedTypes := codegen.NewStringSet()
			var propCount func(string) int
			propCount = func(typeName string) int {
				if visitedTypes.Has(typeName) {
					return 0
				}
				visitedTypes.Add(typeName)
				t := sch.Types[typeName]
				result := len(t.Properties)
				for _, p := range t.Properties {
					if p.Ref != "" {
						tn := strings.TrimPrefix(p.Ref, "#/types/")
						result += propCount(tn)
					}
				}
				return result
			}
			properties := 0
			for n, r := range sch.Resources {
				baseName := versionlessName(n)
				if uniques.Has(baseName) {
					continue
				}
				uniques.Add(baseName)
				properties += len(r.InputProperties)
				for _, p := range r.InputProperties {
					if p.Ref != "" {
						typeName := strings.TrimPrefix(p.Ref, "#/types/")
						properties += propCount(typeName)
					}
				}
			}

			fmt.Printf("Provider: %s\n", provider)
			fmt.Printf("Total resource types: %d\n", len(uniques))
			fmt.Printf("Total input properties: %d\n", properties)

			return nil
		},
	}

	return command
}

func versionlessName(name string) string {
	parts := strings.Split(name, ":")
	mod := parts[1]
	modParts := strings.Split(mod, "/")
	if len(modParts) == 2 {
		mod = modParts[0]
	}
	return fmt.Sprintf("%s:%s", mod, parts[2])
}
