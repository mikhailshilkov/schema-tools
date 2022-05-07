package compare

import (
	"encoding/json"
	"fmt"
	"github.com/mikhailshilkov/schema-tools/pkg"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
)

func Command() *cobra.Command {
	var provider string
	var oldCommit string
	var newCommit string
	var schemaPath string
	command := &cobra.Command{
		Use:   "compare",
		Short: "Compare 2 different schema versions and return the differences",
		Long:  `Get the current version of schema-tools`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// we want to get the provider, the old commit the new commit and have the ability to set the schema directory

			if schemaPath == "" {
				schemaPath = fmt.Sprintf("provider/cmd/pulumi-resource-%s", provider)
			}

			schemaUrlOld := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/%s/schema.json", provider, oldCommit, schemaPath)
			schOld := pkg.DownloadSchema(schemaUrlOld)

			var schNew schema.PackageSpec

			if newCommit == "--local" {
				usr, _ := user.Current()
				basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
				path := fmt.Sprintf("pulumi-%s/%s", provider, schemaPath)
				schemaPath := filepath.Join(basePath, path, "schema.json")
				schNew = pkg.LoadLocalPackageSpec(schemaPath)
			} else if strings.HasPrefix(newCommit, "--local-path=") {
				parts := strings.Split(newCommit, "=")
				schemaPath, err := filepath.Abs(parts[1])
				if err != nil {
					panic("unable to construct absolute path to schema.json")
				}
				schNew = pkg.LoadLocalPackageSpec(schemaPath)
			} else {
				schemaUrlNew := fmt.Sprintf("https://raw.githubusercontent.com/pulumi/pulumi-%s/%s/%s/schema.json", provider, newCommit, schemaPath)
				schNew = pkg.DownloadSchema(schemaUrlNew)
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

			return nil
		},
	}

	// we want to get the provider, the old commit the new commit and have the ability to set the schema directory
	command.PersistentFlags().StringVarP(&provider, "provider", "p", "", "the name of the provider")
	command.PersistentFlags().StringVarP(&oldCommit, "old-commit", "o", "master",
		"the old commit to compare with (defaults to master)")
	command.PersistentFlags().StringVarP(&newCommit, "new-commit", "n", "",
		"the new commit to compare against the old commit")
	command.PersistentFlags().StringVarP(&schemaPath, "schema-path", "s", "",
		"the relative path to the schema file. Defaults to provider/cmd/pulumi-resource-<provider-name>")

	command.MarkPersistentFlagRequired("provider")

	return command
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

func formatName(provider, s string) string {
	return strings.ReplaceAll(strings.TrimPrefix(s, fmt.Sprintf("%s:", provider)), ":", ".")
}
