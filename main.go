package main

import (
	"encoding/json"
	"fmt"
	"github.com/mikhailshilkov/schema-tools/version"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	action := os.Args[1]
	switch action {
	case "compare":
		compare(os.Args[2:])
	case "stats":
		stats(os.Args[2:])
	case "version":
		fmt.Println(version.Version)
	default:
		panic(fmt.Sprintf("Unknown command %+v", os.Args))
	}
}

func compare(args []string) {
	provider := args[0]
	oldCommit := args[1]
	newCommit := args[2]
	//provider := "aws"
	//commit := "4379b20d1aab018bac69c6d86c4219b08f8d3ec4"
	//provider := "azure"
	//oldCommit := "eb5c7e3716d351612536cee5a8dc66c221d78a4f"
	//commit := "5336a0db038ef14201454c4480ac2b9b67d0d41a"
	//usr, _ := user.Current()
	//basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
	//path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s", provider)
	//schemaPathNew := filepath.Join(basePath, path, "schema.json")
	//schNew := readSchema(schemaPathNew)
	//schemaPathOld := filepath.Join(basePath, path, "schema-master.json")
	//schOld := readSchema(schemaPathOld)

	schemaUrlOld := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/provider/cmd/pulumi-resource-%[1]s/schema.json", provider, oldCommit)
	schOld := downloadSchema(schemaUrlOld)

	var schNew schema.PackageSpec

	if newCommit == "--local" {
		usr, _ := user.Current()
		basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
		path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s", provider)
		schemaPath := filepath.Join(basePath, path, "schema.json")
		schNew = loadLocalPackageSpec(schemaPath)
	} else if strings.HasPrefix(newCommit, "--local-path=") {
		parts := strings.Split(newCommit, "=")
		schemaPath, err := filepath.Abs(parts[1])
		if err != nil {
			panic("unable to construct absolute path to schema.json")
		}
		schNew = loadLocalPackageSpec(schemaPath)
	} else {
		schemaUrlNew := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/provider/cmd/pulumi-resource-%[1]s/schema.json", provider, newCommit)
		schNew = downloadSchema(schemaUrlNew)
	}

	var violations []string
	for resName, res := range schOld.Resources {
		newRes, ok := schNew.Resources[resName]
		if !ok {
			violations = append(violations, fmt.Sprintf("Resource %q missing", resName))
			continue
		}

		for propName, prop := range res.InputProperties {
			newProp, ok := newRes.InputProperties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Resource %q missing input %q", resName, propName))
				continue
			}

			vs := validateTypes(&prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q input %q", resName, propName))
			violations = append(violations, vs...)
		}

		for propName, prop := range res.Properties {
			newProp, ok := newRes.Properties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Resource %q missing output %q", resName, propName))
				continue
			}

			vs := validateTypes(&prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q output %q", resName, propName))
			violations = append(violations, vs...)
		}
	}

	for funcName, f := range schOld.Functions {
		newFunc, ok := schNew.Functions[funcName]
		if !ok {
			violations = append(violations, fmt.Sprintf("Function %q missing", funcName))
			continue
		}

		if f.Inputs == nil {
			continue
		}

		for propName, prop := range f.Inputs.Properties {
			newProp, ok := newFunc.Inputs.Properties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Function %q missing input %q", funcName, propName))
				continue
			}

			vs := validateTypes(&prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Function %q input %q", funcName, propName))
			violations = append(violations, vs...)
		}

		for propName, prop := range f.Outputs.Properties {
			newProp, ok := newFunc.Outputs.Properties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Function %q missing output %q", funcName, propName))
				continue
			}

			vs := validateTypes(&prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Function %q output %q", funcName, propName))
			violations = append(violations, vs...)
		}
	}

	for typName, typ := range schOld.Types {
		newTyp, ok := schNew.Types[typName]
		if !ok {
			violations = append(violations, fmt.Sprintf("Type %q missing", typName))
			continue
		}

		for propName, prop := range typ.Properties {
			newProp, ok := newTyp.Properties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Type %q missing property %q", typName, propName))
				continue
			}

			vs := validateTypes(&prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Type %q input %q", typName, propName))
			violations = append(violations, vs...)
		}
	}

	switch len(violations) {
	case 0:
		fmt.Println("Looking good! No breaking changes found.")
	case 1:
		fmt.Println("Found 1 breaking change:")
	default:
		fmt.Printf("Found %d breaking changes:\n", len(violations))
	}

	var violationDetails []string
	if len(violations) > 500 {
		violationDetails = violations[0:499]
	} else {
		violationDetails = violations
	}

	for _, v := range violationDetails {
		fmt.Println(v)
	}

	var newResources []string
	for resName := range schNew.Resources {
		if _, ok := schOld.Resources[resName]; !ok {
			newResources = append(newResources, resName)
		}
	}
	for resName := range schNew.Functions {
		if _, ok := schOld.Functions[resName]; !ok {
			newResources = append(newResources, resName)
		}
	}

	if len(newResources) > 0 {
		fmt.Println("\nNew resources/functions:")
		sort.Strings(newResources)
		for _, v := range newResources {
			fmt.Println(v)
		}
	} else {
		fmt.Println("No new resources/functions.")
	}

	if provider == "azure-native" {
		compareAzureMetadata(args[1:])
	}
}

func validateTypes(old *schema.TypeSpec, new *schema.TypeSpec, prefix string) (violations []string) {
	switch {
	case old == nil && new == nil:
		return
	case old != nil && new == nil:
		violations = append(violations, fmt.Sprintf("had no type but now has %+v", new))
		return
	case old == nil && new != nil:
		violations = append(violations, fmt.Sprintf("had %+v but now has no type", new))
		return
	}

	if old.Type != new.Type {
		violations = append(violations, fmt.Sprintf("%s type changed from %q to %q", prefix, old.Type, new.Type))
	}
	if old.Ref != new.Ref {
		violations = append(violations, fmt.Sprintf("%s type changed from %q to %q", prefix, old.Ref, new.Ref))
	}
	violations = append(violations, validateTypes(old.Items, new.Items, prefix+" items")...)
	violations = append(violations, validateTypes(old.AdditionalProperties, new.AdditionalProperties, prefix+" additional properties")...)
	return
}

func stats(args []string) {
	provider := args[0]
	schemaUrl := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/master/provider/cmd/pulumi-resource-%[1]s/schema.json", provider)
	sch := downloadSchema(schemaUrl)

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
}

func downloadSchema(schemaUrl string) schema.PackageSpec {
	resp, err := http.Get(schemaUrl)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var sch schema.PackageSpec
	if err = json.Unmarshal(body, &sch); err != nil {
		panic(err)
	}

	return sch
}

func loadLocalPackageSpec(filePath string) schema.PackageSpec {
	body, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var sch schema.PackageSpec
	if err = json.Unmarshal(body, &sch); err != nil {
		panic(err)
	}

	return sch
}

func compareAzureMetadata(args []string) {
	provider := "azure-native"
	oldCommit := args[0]
	newCommit := args[1]

	metaUrlOld := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/provider/cmd/pulumi-resource-%[1]s/metadata.json", provider, oldCommit)
	metaOld := downloadAzureMeta(metaUrlOld)

	var metaNew azureAPIMetadata
	if newCommit == "--local" {
		usr, _ := user.Current()
		basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
		path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s", provider)
		metaPath := filepath.Join(basePath, path, "metadata.json")
		metaNew = loadLocalAzureMeta(metaPath)
	} else if strings.HasPrefix(newCommit, "--local-path=") {
		path := strings.Replace(strings.Split(newCommit, "=")[1], "schema.json", "metadata.json", 1)
		metaPath, err := filepath.Abs(path)
		if err != nil {
			panic("unable to construct absolute path to schema.json")
		}
		metaNew = loadLocalAzureMeta(metaPath)
	} else {
		metaUrl := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/provider/cmd/pulumi-resource-%[1]s/metadata.json", provider, newCommit)
		metaNew = downloadAzureMeta(metaUrl)
	}

	var changes []string
	for resName, res := range metaOld.Resources {
		newRes, ok := metaNew.Resources[resName]
		if !ok {
			changes = append(changes, fmt.Sprintf("Resource %q missing", resName))
			continue
		}

		if res.APIVersion != newRes.APIVersion {
			changes = append(changes, fmt.Sprintf("Resource %q changed from %s to %s", resName, res.APIVersion, newRes.APIVersion))
		}
	}

	for funcName, f := range metaOld.Invokes {
		newFunc, ok := metaNew.Invokes[funcName]
		if !ok {
			changes = append(changes, fmt.Sprintf("Function %q missing", funcName))
			continue
		}

		if f.APIVersion != newFunc.APIVersion {
			changes = append(changes, fmt.Sprintf("Resource %q changed from %s to %s", funcName, f.APIVersion, newFunc.APIVersion))
		}
	}

	switch len(changes) {
	case 0:
		fmt.Println("Looking good! No API changes found.")
		return
	case 1:
		fmt.Println("Found 1 API change:")
	default:
		fmt.Printf("Found %d API changes:\n", len(changes))
	}

	for _, v := range changes {
		fmt.Println(v)
	}
}

func downloadAzureMeta(schemaUrl string) azureAPIMetadata {
	resp, err := http.Get(schemaUrl)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var meta azureAPIMetadata
	if err = json.Unmarshal(body, &meta); err != nil {
		panic(err)
	}

	return meta
}

func loadLocalAzureMeta(filePath string) azureAPIMetadata {
	body, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var meta azureAPIMetadata
	if err = json.Unmarshal(body, &meta); err != nil {
		panic(err)
	}

	return meta
}

type azureAPIMetadata struct {
	Resources map[string]azureAPIResource `json:"resources"`
	Invokes   map[string]azureAPIInvoke   `json:"invokes"`
}

type azureAPIResource struct {
	APIVersion string `json:"apiVersion"`
}

type azureAPIInvoke struct {
	APIVersion string `json:"apiVersion"`
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
