// Code generated by ""fitask" -type=EBSVolume"; DO NOT EDIT

package awstasks

import (
	"encoding/json"

	"k8s.io/kube-deploy/upup/pkg/fi"
)

// EBSVolume

// JSON marshalling boilerplate
type realEBSVolume EBSVolume

func (o *EBSVolume) UnmarshalJSON(data []byte) error {
	var jsonName string
	if err := json.Unmarshal(data, &jsonName); err == nil {
		o.Name = &jsonName
		return nil
	}

	var r realEBSVolume
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*o = EBSVolume(r)
	return nil
}

var _ fi.CompareWithID = &EBSVolume{}

func (e *EBSVolume) CompareWithID() *string {
	return e.ID
}

var _ fi.HasName = &EBSVolume{}

func (e *EBSVolume) GetName() *string {
	return e.Name
}

func (e *EBSVolume) SetName(name string) {
	e.Name = &name
}

func (e *EBSVolume) String() string {
	return fi.TaskAsString(e)
}
