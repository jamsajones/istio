// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bootstrapgen

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"golang.org/x/tools/imports"

	"istio.io/api/mixer/v1/config/descriptor"
	tmplPkg "istio.io/istio/mixer/tools/codegen/pkg/bootstrapgen/template"
	"istio.io/istio/mixer/tools/codegen/pkg/modelgen"
)

// Generator creates a Go file that will be build inside mixer framework. The generated file contains all the
// template specific code that mixer needs to add support for different passed in templates.
type Generator struct {
	OutFilePath   string
	ImportMapping map[string]string
}

const (
	fullGoNameOfValueTypePkgName = "istio_mixer_v1_config_descriptor."
)

// TODO share the code between this generator and the interfacegen code generator.
var primitiveToValueType = map[string]string{
	"string":  fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.STRING.String(),
	"bool":    fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.BOOL.String(),
	"int64":   fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.INT64.String(),
	"float64": fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.DOUBLE.String(),
	// TODO: currently IP_ADDRESS is byte[], but reverse might not be true. This code assumes []byte is
	// IP_ADDRESS, which is a temporary hack since there is currently no way to express IP_ADDRESS inside templates
	// yet.
	"[]byte":        fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.IP_ADDRESS.String(),
	"time.Duration": fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.DURATION.String(),
	"time.Time":     fullGoNameOfValueTypePkgName + istio_mixer_v1_config_descriptor.TIMESTAMP.String(),
}

func containsValueTypeOrResMsg(ti modelgen.TypeInfo) bool {
	return ti.IsValueType || ti.IsResourceMessage || ti.IsMap && (ti.MapValue.IsValueType || ti.MapValue.IsResourceMessage)
}

type bootstrapModel struct {
	PkgName        string
	TemplateModels []*modelgen.Model
}

const goImportFmt = "\"%s\""
const resourceMsgTypeSuffix = "Type"
const resourceMsgInstanceSuffix = "Instance"
const resourceMsgInstParamSuffix = "InstanceParam"
const templateName = "Template"

// Generate creates a Go file that will be build inside mixer framework. The generated file contains all the
// template specific code that mixer needs to add support for different passed in templates.
func (g *Generator) Generate(fdsFiles map[string]string) error {
	imprts := make([]string, 0)
	tmpl, err := template.New("MixerBootstrap").Funcs(
		template.FuncMap{
			"getValueType": func(goType modelgen.TypeInfo) string {
				return primitiveToValueType[goType.Name]

			},
			"containsValueTypeOrResMsg": containsValueTypeOrResMsg,
			"reportTypeUsed": func(ti modelgen.TypeInfo) string {
				if len(ti.Import) > 0 {
					imprt := fmt.Sprintf(goImportFmt, ti.Import)
					if !contains(imprts, imprt) {
						imprts = append(imprts, imprt)
					}
				}
				// do nothing, just record the import so that we can add them later (only for the types that got printed)
				return ""
			},
			"getResourcMessageTypeName": func(s string) string {
				if s == templateName {
					return resourceMsgTypeSuffix
				}
				return s + resourceMsgTypeSuffix
			},
			"getResourcMessageInstanceName": func(s string) string {
				if s == templateName {
					return resourceMsgInstanceSuffix
				}
				return s
			},

			"getResourcMessageInterfaceParamTypeName": func(s string) string {
				if s == templateName {
					return resourceMsgInstParamSuffix
				}
				return s + resourceMsgInstParamSuffix
			},
			"getAllMsgs": func(model modelgen.Model) []modelgen.MessageInfo {
				res := make([]modelgen.MessageInfo, 0)
				res = append(res, model.TemplateMessage)
				res = append(res, model.ResourceMessages...)
				return res
			},
			"getTypeName": func(goType modelgen.TypeInfo) string {
				// GoType for a Resource message has a pointer reference. Therefore for a raw type name, we should strip
				// the "*".
				return strings.Trim(goType.Name, "*")
			},
			"getBuildFnName": func(typeName string) string {
				return "Build" + typeName
			},
		}).Parse(tmplPkg.InterfaceTemplate)

	if err != nil {
		return fmt.Errorf("cannot load template: %v", err)
	}

	models := make([]*modelgen.Model, 0)
	var fdss []string
	for k := range fdsFiles {
		fdss = append(fdss, k)
	}
	sort.Strings(fdss)

	for _, fdsPath := range fdss {
		var fds *descriptor.FileDescriptorSet
		fds, err = getFileDescSet(fdsPath)
		if err != nil {
			return fmt.Errorf("cannot parse file '%s' as a FileDescriptorSetProto. %v", fds, err)
		}

		var parser *modelgen.FileDescriptorSetParser
		parser, err = modelgen.CreateFileDescriptorSetParser(fds, g.ImportMapping, fdsFiles[fdsPath])
		if err != nil {
			return fmt.Errorf("cannot parse file '%s' as a FileDescriptorSetProto. %v", fds, err)
		}

		var model *modelgen.Model
		if model, err = modelgen.Create(parser); err != nil {
			return err
		}

		// TODO validate there is no ambiguity in template names.
		models = append(models, model)
	}

	pkgName := getParentDirName(g.OutFilePath)

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, bootstrapModel{pkgName, models})
	if err != nil {
		return fmt.Errorf("cannot execute the template with the given data: %v", err)
	}
	bytesWithImpts := bytes.Replace(buf.Bytes(), []byte("$$additional_imports$$"), []byte(strings.Join(imprts, "\n")), 1)
	fmtd, err := format.Source(bytesWithImpts)
	if err != nil {
		return fmt.Errorf("could not format generated code: %v. Source code is %s", err, buf.String())
	}

	imports.LocalPrefix = "istio.io"
	// OutFilePath provides context for import path. We rely on the supplied bytes for content.
	imptd, err := imports.Process(g.OutFilePath, fmtd, &imports.Options{FormatOnly: true, Comments: true})
	if err != nil {
		return fmt.Errorf("could not fix imports for generated code: %v", err)
	}

	f, err := os.Create(g.OutFilePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // nolint: gas
	if _, err = f.Write(imptd); err != nil {
		_ = f.Close()           // nolint: gas
		_ = os.Remove(f.Name()) // nolint: gas
		return err
	}
	return nil
}

func getParentDirName(filePath string) string {
	return filepath.Base(filepath.Dir(filePath))
}

func getFileDescSet(path string) (*descriptor.FileDescriptorSet, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fds := &descriptor.FileDescriptorSet{}
	err = proto.Unmarshal(bytes, fds)
	if err != nil {
		return nil, err
	}

	return fds, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
