package thirdparty

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

var rules interpreter.Rules = interpreter.AllResourceInterpreterCustomizationRules

func checkScript(script string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	l, err := luavm.NewWithContext(ctx)
	if err != nil {
		return err
	}
	defer l.Close()
	_, err = l.LoadString(script)
	return err
}

func getObj(t *testing.T, path string) *unstructured.Unstructured {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := yaml.ToJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	obj := make(map[string]interface{})
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func getAggregatedStatusItems(t *testing.T, path string) []workv1alpha2.AggregatedStatusItem {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var statusItems []workv1alpha2.AggregatedStatusItem
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		statusItem := &workv1alpha2.AggregatedStatusItem{}
		err = decoder.Decode(statusItem)
		if err != nil {
			break
		}
		statusItems = append(statusItems, *statusItem)
	}
	if err != io.EOF {
		t.Fatal(err)
	}

	return statusItems
}

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	DesiredInputPath  string `yaml:"desiredInputPath,omitempty"`
	ObservedInputPath string `yaml:"observedInputPath,omitempty"`
	StatusInputPath   string `yaml:"statusInputPath,omitempty"`
	DesiredReplica    int64  `yaml:"desiredReplicas,omitempty"`
	Operation         string `yaml:"operation"`
}

func checkInterpretationRule(t *testing.T, path string, configs []*configv1alpha1.ResourceInterpreterCustomization) {
	ipt := declarative.NewConfigurableInterpreter(nil)
	ipt.LoadConfig(configs)

	dir := filepath.Dir(path)
	yamlBytes, err := os.ReadFile(dir + string(os.PathSeparator) + "customizations_tests.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var resourceTest TestStructure
	err = yaml.Unmarshal(yamlBytes, &resourceTest)
	if err != nil {
		t.Fatal(err)
	}
	for _, customization := range configs {
		for _, input := range resourceTest.Tests {
			rule := rules.GetByOperation(input.Operation)
			if rule == nil {
				t.Fatalf("operation %s is not supported. Use one of: %s", input.Operation, strings.Join(rules.Names(), ", "))
			}
			err = checkScript(rule.GetScript(customization))
			if err != nil {
				t.Fatalf("checking %s of %s, expected nil, but got: %v", rule.Name(), customization.Name, err)
			}
			args := interpreter.RuleArgs{Replica: input.DesiredReplica}
			if input.DesiredInputPath != "" {
				args.Desired = getObj(t, dir+"/"+strings.TrimPrefix(input.DesiredInputPath, "/"))
			}
			if input.ObservedInputPath != "" {
				args.Observed = getObj(t, dir+"/"+strings.TrimPrefix(input.ObservedInputPath, "/"))
			}
			if input.StatusInputPath != "" {
				args.Status = getAggregatedStatusItems(t, dir+"/"+strings.TrimPrefix(input.StatusInputPath, "/"))
			}
			if result := rule.Run(ipt, args); result.Err != nil {
				t.Fatalf("execute %s %s error: %v\n", customization.Name, rule.Name(), result.Err)
			}
		}
	}
}

func TestThirdPartyCustomizationsFile(t *testing.T) {
	err := filepath.Walk("resourcecustomizations", func(path string, f os.FileInfo, err error) error {
		if err != nil {
			// cannot happen
			return err
		}
		if f.IsDir() {
			return nil
		}
		if strings.Contains(path, "testdata") {
			return nil
		}
		if filepath.Base(path) != configurableInterpreterFile {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			// cannot happen
			return err
		}
		var configs []*configv1alpha1.ResourceInterpreterCustomization
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			config := &configv1alpha1.ResourceInterpreterCustomization{}
			err = decoder.Decode(config)
			if err != nil {
				break
			}
			dirSplit := strings.Split(path, string(os.PathSeparator))
			if len(dirSplit) != 5 {
				return fmt.Errorf("the directory format is incorrect. Dir: %s", path)
			}
			if config.Spec.Target.APIVersion != fmt.Sprintf("%s/%s", dirSplit[1], dirSplit[2]) {
				return fmt.Errorf("Target.APIVersion does not match directory format. Target.APIVersion: %s, Dir: %s", config.Spec.Target.APIVersion, path)
			}
			if config.Spec.Target.Kind != dirSplit[3] {
				return fmt.Errorf("Target.Kind does not match directory format. Target.Kind: %s, Dir: %s", config.Spec.Target.Kind, path)
			}
			configs = append(configs, config)
		}
		if err != io.EOF {
			return err
		}
		checkInterpretationRule(t, path, configs)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, but got: %v", err)
	}
}
