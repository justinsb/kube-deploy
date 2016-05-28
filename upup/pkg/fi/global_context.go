package fi

//import "sync"
//
//var globalContextMap map[string]*GlobalContextState
//var globalContextMapMutex sync.Mutex
//
//type GlobalContextState struct {
//	key       string
//
//	mutex     sync.Mutex
//	readMutex sync.Mutex
//	context   interface{}
//}
//
//func (s*GlobalContextState) runInGlobalContext(context interface{}, fn func() error) error {
//	s.mutex.Lock()
//	defer func() {
//		s.context = nil
//		s.mutex.Unlock()
//	}()
//
//	s.readMutex.Lock()
//	s.context = context
//	defer s.readMutex.Unlock()
//
//	return fn()
//}
//
//func (s*GlobalContextState) getContext() interface{} {
//	s.readMutex.Lock()
//	defer s.readMutex.Unlock()
//
//	return s.context
//}
//
//func getGlobalContextState(key string) *GlobalContextState {
//	globalContextMapMutex.Lock()
//	defer globalContextMapMutex.Unlock()
//
//	state := globalContextMap[key]
//	if state == nil {
//		state = &GlobalContextState{
//			key: key,
//		}
//		globalContextMap[key] = state
//	}
//	return state
//}
//
//func RunInGlobalContext(key string, context interface{}, fn func() error) error {
//	state := getGlobalContextState(key)
//	return state.runInGlobalContext(context, fn)
//}
//
//func GetGlobalContext(key string) interface{} {
//	state := getGlobalContextState(key)
//	return state.getContext()
//}