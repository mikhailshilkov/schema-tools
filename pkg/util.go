package pkg

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func DownloadSchema(schemaUrl string) schema.PackageSpec {
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

func LoadLocalPackageSpec(filePath string) schema.PackageSpec {
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
