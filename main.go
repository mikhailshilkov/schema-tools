package main

import (
	"encoding/json"
	"fmt"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"
)

func main() {
	usr, _ := user.Current()
	basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)

	providers := []string {"aws", "aws-native", "azure", "azure-native", "gcp", "gcp-native"}
	for _, provider := range providers {
		path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s/schema.json", provider)
		schemaPath := filepath.Join(basePath, path)
		sch := readSchema(schemaPath)

		uniques := codegen.NewStringSet()
		for n := range sch.Resources {
			uniques.Add(versionlessName(n))
		}

		fmt.Println("------")
		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("Total resources: %d\n", len(sch.Resources))
		fmt.Printf("Unique resources: %d\n", len(uniques))
	}
}

func readSchema(schemaPath string) schema.PackageSpec {
	schemaBytes, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		panic(err)
	}

	var sch schema.PackageSpec
	if err = json.Unmarshal(schemaBytes, &sch); err != nil {
		panic(err)
	}

	return sch
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
