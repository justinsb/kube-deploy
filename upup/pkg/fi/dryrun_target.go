package fi

import (
	"fmt"

	"bytes"
	"github.com/golang/glog"
	"io"
	"k8s.io/kube-deploy/upup/pkg/fi/utils"
	"reflect"
)

// DryRunTarget is a special Target that does not execute anything, but instead tracks all changes.
// By running against a DryRunTarget, a list of changes that would be made can be easily collected,
// without any special support from the Tasks.
type DryRunTarget struct {
	changes []*render

	// The destination to which the final report will be printed on Finish()
	out io.Writer
}

type render struct {
	a       Task
	aIsNil  bool
	e       Task
	changes Task
}

var _ Target = &DryRunTarget{}

func NewDryRunTarget(out io.Writer) *DryRunTarget {
	t := &DryRunTarget{}
	t.out = out
	return t
}

func (t *DryRunTarget) Render(a, e, changes Task) error {
	valA := reflect.ValueOf(a)
	aIsNil := valA.IsNil()

	t.changes = append(t.changes, &render{
		a:       a,
		aIsNil:  aIsNil,
		e:       e,
		changes: changes,
	})
	return nil
}

func IdForTask(taskMap map[string]Task, t Task) string {
	for k, v := range taskMap {
		if v == t {
			return k
		}
	}
	glog.Fatalf("unknown task: %v", t)
	return "?"
}

func (t *DryRunTarget) PrintReport(taskMap map[string]Task, out io.Writer) error {
	b := &bytes.Buffer{}

	if len(t.changes) != 0 {
		fmt.Fprintf(b, "Created resources:\n")
		for _, r := range t.changes {
			if !r.aIsNil {
				continue
			}

			fmt.Fprintf(b, "  %T\t%s\n", r.changes, IdForTask(taskMap, r.e))
		}

		fmt.Fprintf(b, "Changed resources:\n")
		// We can't use our reflection helpers here - we want corresponding values from a,e,c
		for _, r := range t.changes {
			if r.aIsNil {
				continue
			}
			var changeList []string

			valC := reflect.ValueOf(r.changes)
			valA := reflect.ValueOf(r.a)
			valE := reflect.ValueOf(r.e)
			if valC.Kind() == reflect.Ptr && !valC.IsNil() {
				valC = valC.Elem()
			}
			if valA.Kind() == reflect.Ptr && !valA.IsNil() {
				valA = valA.Elem()
			}
			if valE.Kind() == reflect.Ptr && !valE.IsNil() {
				valE = valE.Elem()
			}
			if valC.Kind() == reflect.Struct {
				for i := 0; i < valC.NumField(); i++ {
					fieldValC := valC.Field(i)

					changed := true
					switch fieldValC.Kind() {
					case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
						changed = !fieldValC.IsNil()

					case reflect.String:
						changed = fieldValC.Interface().(string) != ""
					}
					if !changed {
						continue
					}

					if fieldValC.Kind() == reflect.String && fieldValC.Interface().(string) == "" {
						// No change
						continue
					}

					fieldValE := valE.Field(i)

					description := ""
					ignored := false
					if fieldValE.CanInterface() {
						fieldValA := valA.Field(i)

						switch fieldValE.Interface().(type) {
						//case SimpleUnit:
						//	ignored = true
						default:
							description = fmt.Sprintf(" %v -> %v", asString(fieldValA), asString(fieldValE))
						}
					}
					if ignored {
						continue
					}
					changeList = append(changeList, valC.Type().Field(i).Name+description)
				}
			} else {
				return fmt.Errorf("unhandled change type: %v", valC.Type())
			}

			if len(changeList) == 0 {
				continue
			}

			fmt.Fprintf(b, "  %T\t%s\n", r.changes, IdForTask(taskMap, r.e))
			for _, f := range changeList {
				fmt.Fprintf(b, "    %s\n", f)
			}
			fmt.Fprintf(b, "\n")
		}
	}

	_, err := out.Write(b.Bytes())
	return err
}

// asString returns a human-readable string representation of the passed value
func asString(value reflect.Value) string {
	b := &bytes.Buffer{}

	walker := func(path string, field *reflect.StructField, v reflect.Value) error {
		if utils.IsPrimitiveValue(v) || v.Kind() == reflect.String {
			fmt.Fprintf(b, "%v", v.Interface())
			return utils.SkipReflection
		}

		switch v.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
			if v.IsNil() {
				fmt.Fprintf(b, "<nil>")
				return utils.SkipReflection
			}
		}

		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			return nil // descend into value

		case reflect.Slice:
			len := v.Len()
			fmt.Fprintf(b, "[")
			for i := 0; i < len; i++ {
				av := v.Index(i)

				if i != 0 {
					fmt.Fprintf(b, ", ")
				}
				fmt.Fprintf(b, "%s", asString(av))
			}
			fmt.Fprintf(b, "]")
			return utils.SkipReflection

		case reflect.Map:
			keys := v.MapKeys()
			fmt.Fprintf(b, "{")
			for i, key := range keys {
				mv := v.MapIndex(key)

				if i != 0 {
					fmt.Fprintf(b, ", ")
				}
				fmt.Fprintf(b, "%s: %s", asString(key), asString(mv))
			}
			fmt.Fprintf(b, "}")
			return utils.SkipReflection

		case reflect.Struct:
			intf := v.Addr().Interface()
			if _, ok := intf.(Resource); ok {
				fmt.Fprintf(b, "<resource>")
			} else if _, ok := intf.(*ResourceHolder); ok {
				fmt.Fprintf(b, "<resource>")
			} else if compareWithID, ok := intf.(CompareWithID); ok {
				id := compareWithID.CompareWithID()
				if id == nil {
					fmt.Fprintf(b, "id:<nil>")
				} else {
					fmt.Fprintf(b, "id:%s", *id)
				}
			} else {
				return fmt.Errorf("Unhandled type for %q: %T", path, intf)
			}
			return utils.SkipReflection

		default:
			glog.Infof("Unhandled kind for %q: %T", path, v.Interface())
			return fmt.Errorf("Unhandled kind for %q: %v", path, v.Kind())
		}
	}

	err := utils.ReflectRecursive(value, walker)
	if err != nil {
		glog.Fatalf("unexpected error during reflective walk: %v", err)
	}
	return b.String()
}

// Finish is called at the end of a run, and prints a list of changes to the configured Writer
func (t *DryRunTarget) Finish(taskMap map[string]Task) error {
	return t.PrintReport(taskMap, t.out)
}
