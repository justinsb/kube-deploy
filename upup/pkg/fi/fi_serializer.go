package fi

//import (
//	"github.com/kopeio/kope/pkg/hidefi"
//	"encoding/json"
//)

//const GlobalContextKey = "FiSerialization"
//
//type deferral struct {
//	link string
//	task Task
//}
//
//type FiSerialization struct {
//	deferrals   []*deferral
//	triedCustom map[interface{}]bool
//}
//
//func getFiSerializationContext() *FiSerialization {
//	return GetGlobalContext(GlobalContextKey).(*FiSerialization)
//}
//
//func UnmarshalFi() {
//	context := &FiSerialization{}
//	err := RunInGlobalContext(GlobalContextKey, context, context.run())
//}
//
//func UnmarshalObjectOrName(data []byte, task interface{}, name *string) error {
//	context := getFiSerializationContext()
//	if context != nil && !context.triedCustom[task] {
//		var jsonLink string
//		if err := json.Unmarshal(data, &jsonLink); err == nil {
//			context.deferrals = append(context.deferrals, &deferral{link:jsonLink, task: task})
//			return nil
//		}
//	}
//
//	context.triedCustom[task] = true
//	return json.Unmarshal(data, task)
//}
//

//
//func UnmarshalObjectOrName(data []byte, task interface{}, name *string) error {
//		var jsonString string
//		if err := json.Unmarshal(data, &jsonString); err == nil {
//			*name = jsonString
//			return nil
//	}
//
//	return json.Unmarshal(data, task)
//}
