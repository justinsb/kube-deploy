package nodeup

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi/utils"
	"reflect"
	"sort"
	"strings"
	"encoding/json"
)

// buildFlags is a template helper, which builds a string containing the flags to be passed to a command
func buildFlags(options interface{}) (string, error) {
	flags, err := buildFlagsSlice(options)
	if err != nil {
		return "", err
	}
	return strings.Join(flags, " "), nil
}

// BuildBootstrap is a template helper, whcih builds a BootstrapTask for use with protokube
func buildBootstrap(command string, options interface{}) (string, error) {
	flags, err := buildFlagsSlice(options)
	if err != nil {
		return "", err
	}
	t := &BootstrapTask{}
	t.Command = append(t.Command, command)
	t.Command = append(t.Command, flags...)

	j, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling to json: %v", err)
	}
	return string(j), nil
}

func buildFlagsSlice(options interface{}) ([]string, error) {
	var flags []string

	walker := func(path string, field *reflect.StructField, val reflect.Value) error {
		if field == nil {
			glog.V(4).Infof("not writing non-field: %s", path)
			return nil
		}
		tag := field.Tag.Get("flag")
		if tag == "" {
			glog.V(4).Infof("not writing field with no flag tag: %s", path)
			return nil
		}
		if tag == "-" {
			glog.V(4).Infof("skipping field with %q flag tag: %s", tag, path)
			return utils.SkipReflection
		}
		flagName := tag

		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return nil
			}
			val = val.Elem()
		}

		var flag string
		switch v := val.Interface().(type) {
		case string, int, bool, float32, float64:
			vString := fmt.Sprintf("%v", v)
			if vString != "" {
				flag = fmt.Sprintf("--%s=%s", flagName, vString)
			}

		default:
			return fmt.Errorf("BuildFlags of value type not handled: %T %s=%v", v, path, v)
		}
		if flag != "" {
			flags = append(flags, flag)
		}
		return nil
	}
	err := utils.ReflectRecursive(reflect.ValueOf(options), walker)
	if err != nil {
		return nil, err
	}
	// Sort so that the order is stable across runs
	sort.Strings(flags)

	return flags, nil
}
