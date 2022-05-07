package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/mikhailshilkov/schema-tools/cmd"
)

func main() {
	flag.Parse()

	defer glog.Flush()

	if err := cmd.RootCmd().Execute(); err != nil {
		glog.Errorf("Failed to execute command: %v", err)
		os.Exit(1)
	}
}

// @Stack72 - I don't know what exp actually is...
//func main() {
//	action := os.Args[1]
//	switch action {
//	case "exp":
//		exp()
//	default:
//		panic(fmt.Sprintf("Unknown command %+v", os.Args))
//	}
//}
//
//func exp() {
//	provider := "azure-native"
//	usr, _ := user.Current()
//	basePath := fmt.Sprintf("%s/go/src/github.com/pulumi", usr.HomeDir)
//	path := fmt.Sprintf("pulumi-%s/provider/cmd/pulumi-resource-%[1]s", provider)
//	schemaPath := filepath.Join(basePath, path, "schema-full.json")
//	sch := loadLocalPackageSpec(schemaPath)
//
//	var violations []string
//	resName := "azure-native:web/v20210101:WebApp"
//	res := sch.Resources[resName]
//	newRes := sch.Resources["azure-native:web/v20201201:WebApp"]
//
//	for propName, prop := range res.InputProperties {
//		newProp, ok := newRes.InputProperties[propName]
//		if !ok {
//			violations = append(violations, fmt.Sprintf("Resource %q missing input %q: %q", resName, propName, newProp.Description))
//			continue
//		}
//
//		vs := validateTypes2(&sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q input %q", resName, propName))
//		violations = append(violations, vs...)
//	}
//
//	for propName, prop := range res.Properties {
//		newProp, ok := newRes.Properties[propName]
//		if !ok {
//			violations = append(violations, fmt.Sprintf("Resource %q missing output %q", resName, propName))
//			continue
//		}
//
//		vs := validateTypes2(&sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Resource %q output %q", resName, propName))
//		violations = append(violations, vs...)
//	}
//
//	for _, v := range violations {
//		fmt.Println(v)
//	}
//	if len(violations) == 0 {
//		fmt.Println("No violations")
//	}
//}
//
//func validateTypes2(sch *schema.PackageSpec, old *schema.TypeSpec, new *schema.TypeSpec, prefix string) (violations []string) {
//	switch {
//	case old == nil && new == nil:
//		return
//	case old != nil && new == nil:
//		violations = append(violations, fmt.Sprintf("had no type but now has %+v", new))
//		return
//	case old == nil && new != nil:
//		violations = append(violations, fmt.Sprintf("had %+v but now has no type", new))
//		return
//	}
//
//	if old.Type != new.Type {
//		violations = append(violations, fmt.Sprintf("%s type changed from %q to %q", prefix, old.Type, new.Type))
//	}
//	if old.Ref != new.Ref {
//		oldType := sch.Types[strings.TrimPrefix(old.Ref, "#/types/")]
//		ntn := strings.TrimPrefix(new.Ref, "#/types/")
//		newType := sch.Types[ntn]
//
//		for propName, prop := range oldType.Properties {
//			newProp, ok := newType.Properties[propName]
//			if !ok {
//				violations = append(violations, fmt.Sprintf("Type %q missing property %q", ntn, propName))
//				continue
//			}
//
//			vs := validateTypes2(sch, &prop.TypeSpec, &newProp.TypeSpec, fmt.Sprintf("Type %q input %q", ntn, propName))
//			violations = append(violations, vs...)
//		}
//	}
//	violations = append(violations, validateTypes2(sch, old.Items, new.Items, prefix+" items")...)
//	violations = append(violations, validateTypes2(sch, old.AdditionalProperties, new.AdditionalProperties, prefix+" additional properties")...)
//	return
//}
