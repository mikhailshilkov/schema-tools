package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mikhailshilkov/schema-tools/version"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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
	case "exp":
		exp()
	default:
		panic(fmt.Sprintf("Unknown command %+v", os.Args))
	}
}

func exp() {
	provider := "azure-native"
	usr, _ := user.Current()
	basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
	path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s", provider)
	schemaPath := filepath.Join(basePath, path, "schema-full.json")
	sch := loadLocalPackageSpec(schemaPath)

	var violations []string
	resName := "azure-native:web/v20210101:WebApp"
	res := sch.Resources[resName]
	newRes := sch.Resources["azure-native:web/v20201201:WebApp"]

	for propName, prop := range res.InputProperties {
		newProp, ok := newRes.InputProperties[propName]
		if !ok {
			violations = append(violations, fmt.Sprintf("Resource %q missing input %q: %q", resName, propName, newProp.Description))
			continue
		}

		vs := validateTypes2(&sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q input %q", resName, propName))
		violations = append(violations, vs...)
	}

	for propName, prop := range res.Properties {
		newProp, ok := newRes.Properties[propName]
		if !ok {
			violations = append(violations, fmt.Sprintf("Resource %q missing output %q", resName, propName))
			continue
		}

		vs := validateTypes2(&sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q output %q", resName, propName))
		violations = append(violations, vs...)
	}

	for _, v := range violations {
		fmt.Println(v)
	}
	if len(violations) == 0 {
		fmt.Println("No violations")
	}
}

func validateTypes2(sch *schema.PackageSpec, old *schema.TypeSpec, new *schema.TypeSpec, prefix string) (violations []string) {
	switch {
	case old == nil && new == nil:
		return
	case old != nil && new == nil:
		violations = append(violations, fmt.Sprintf("had %+v but now has no type", old))
		return
	case old == nil && new != nil:
		violations = append(violations, fmt.Sprintf("had no type but now has %+v", new))
		return
	}

	if old.Type != new.Type {
		violations = append(violations, fmt.Sprintf("%s type changed from %q to %q", prefix, old.Type, new.Type))
	}
	if old.Ref != new.Ref {
		oldType := sch.Types[strings.TrimPrefix(old.Ref, "#/types/")]
		ntn := strings.TrimPrefix(new.Ref, "#/types/")
		newType := sch.Types[ntn]

		for propName, prop := range oldType.Properties {
			newProp, ok := newType.Properties[propName]
			if !ok {
				violations = append(violations, fmt.Sprintf("Type %q missing property %q", ntn, propName))
				continue
			}

			vs := validateTypes2(sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Type %q input %q", ntn, propName))
			violations = append(violations, vs...)
		}
	}
	violations = append(violations, validateTypes2(sch, old.Items, new.Items, prefix+" items")...)
	violations = append(violations, validateTypes2(sch, old.AdditionalProperties, new.AdditionalProperties, prefix+" additional properties")...)
	return
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

	var org string
	if len(args) == 4 {
		org = args[3]
	} else {
		org = "pulumi"
	}

	schemaUrlOld := fmt.Sprintf("https://raw.githubusercontent.com/%s/pulumi-%s/%s/provider/cmd/pulumi-resource-%[2]s/schema.json", org, provider, oldCommit)
	fmt.Println(schemaUrlOld)
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
		schemaUrlNew := fmt.Sprintf("https://raw.githubusercontent.com/%s/pulumi-%s/%s/provider/cmd/pulumi-resource-%[2]s/schema.json", org, provider, newCommit)
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
			if newFunc.Inputs == nil {
				violations = append(violations, fmt.Sprintf("Function %q missing input %q", funcName, propName))
				continue
			}

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

	var newResources, newFunctions []string
	for resName := range schNew.Resources {
		if _, ok := schOld.Resources[resName]; !ok {
			newResources = append(newResources, formatName(provider, resName))
		}
	}
	for resName := range schNew.Functions {
		if _, ok := schOld.Functions[resName]; !ok {
			newFunctions = append(newFunctions, formatName(provider, resName))
		}
	}

	if len(newResources) > 0 {
		fmt.Println("\n#### New resources:\n")
		sort.Strings(newResources)
		for _, v := range newResources {
			fmt.Printf("- `%s`\n", v)
		}
	}

	if len(newFunctions) > 0 {
		fmt.Println("\n#### New functions:\n")
		sort.Strings(newFunctions)
		for _, v := range newFunctions {
			fmt.Printf("- `%s`\n", v)
		}
	}

	if len(newResources) == 0 && len(newFunctions) == 0 {
		fmt.Println("No new resources/functions.")
	}

	if provider == "azure-native" {
		compareAzureMetadata(args[1:])
	}
}

func formatName(provider, s string) string {
	return strings.ReplaceAll(strings.TrimPrefix(s, fmt.Sprintf("%s:", provider)), ":", ".")
}

func validateTypes(old *schema.TypeSpec, new *schema.TypeSpec, prefix string) (violations []string) {
	switch {
	case old == nil && new == nil:
		return
	case old != nil && new == nil:
		violations = append(violations, fmt.Sprintf("had %+v but now has no type", old))
		return
	case old == nil && new != nil:
		violations = append(violations, fmt.Sprintf("had no type but now has %+v", new))
		return
	}

	oldType := old.Type
	if old.Ref != "" {
		oldType = old.Ref
	}
	newType := new.Type
	if new.Ref != "" {
		newType = new.Ref
	}
	if oldType != newType {
		violations = append(violations, fmt.Sprintf("%s type changed from %q to %q", prefix, oldType, newType))
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
			changes = append(changes, fmt.Sprintf("Change in %q from %s to %s", resName, res.APIVersion, newRes.APIVersion))
		}
	}

	for funcName, f := range metaOld.Invokes {
		newFunc, ok := metaNew.Invokes[funcName]
		if !ok {
			changes = append(changes, fmt.Sprintf("Function %q missing", funcName))
			continue
		}

		if f.APIVersion != newFunc.APIVersion {
			changes = append(changes, fmt.Sprintf("Change in function %q from %s to %s", funcName, f.APIVersion, newFunc.APIVersion))
		}
	}

	for resName := range metaNew.Resources {
		if _, ok := metaOld.Resources[resName]; !ok {
			changes = append(changes, fmt.Sprintf("New resource %q", resName))
		}
	}

	fmt.Println()
	sort.Strings(changes)
	switch len(changes) {
	case 0:
		fmt.Println("Looking good! No API changes found.")
		return
	case 1:
		fmt.Println("#### Found 1 API change:\n")
	default:
		fmt.Printf("#### Found %d API changes:\n\n", len(changes))
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
