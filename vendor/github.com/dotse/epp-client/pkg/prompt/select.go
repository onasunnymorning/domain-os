// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package prompt

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/dotse/epp-client/pkg"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
)

const (
	done      = "Done"
	exit      = "Exit"
	customXML = "Custom XML"
)

func (p *Prompt) setupRequest(commands []*pkg.Command) (name, template string, data any) {
	options := []string{}
	selections := map[string]*pkg.Command{}

	for _, c := range commands {
		options = append(options, c.Name)
		selections[c.Name] = c
	}

	options = append(options, customXML, exit)

	pu := promptui.Select{
		Label: "Select Command",
		Items: options,
		Size:  15,
	}

	_, choice, err := pu.Run()
	if err != nil {
		panic(err)
	}

	switch choice {
	case exit:
		return choice, "", nil
	case customXML:
		return choice, "", p.requestCustomXML()
	}

	command := selections[choice]
	if command.SubCommand == nil {
		// we have found a leaf
		cmdDta, send := p.setupCommandData(*command)
		if send {
			// the user doesn't want to edit more data, send the request.
			return command.Name, command.Template, cmdDta
		}

		// update more command data
		return p.setupRequest(commands)
	}

	// not a leaf yet, keep looking
	name, template, data = p.setupRequest(command.SubCommand)
	switch name {
	case exit:
		// go back
		return p.setupRequest(commands)
	case customXML:
		return name, template, data
	}

	return name, template, data
}

// requestCustomXML ask the user to insert the custom XML to send to the server.
func (p *Prompt) requestCustomXML() []byte {
	p.Cli.Println("Put you XML here, end with two new lines")

	if !p.MultilineScanner.Scan() {
		panic(p.MultilineScanner.Err())
	}

	return p.MultilineScanner.Bytes()
}

// setupCommandData ask the user to update the command data to send to the server.
func (p *Prompt) setupCommandData(command pkg.Command) (any, bool) {
	options := []string{"Send request", "Show request", "Validate request"}
	data := map[string]*FieldValue{}

	if command.DefaultData != nil {
		// print the current data for the command
		for _, v := range GetFieldValues(command.DefaultData) {
			options = append(options, fmt.Sprintf("%s (%s): %s", v.Name, v.Type, v.ValueAsString()))
			data[v.Name] = v
		}
	}

	options = append(options, exit)

	pu := promptui.Select{
		Label: "Current data",
		Items: options,
		Size:  15,
	}

	_, choice, err := pu.Run()
	if err != nil {
		panic(err)
	}

	switch choice {
	case exit:
		return command.DefaultData, false

	case "Show request":
		p.printRequest(os.Stdout, command.Template, command.DefaultData)

		// see if the user want to update some more data
		return p.setupCommandData(command)

	case "Send request":
		return command.DefaultData, true

	case "Validate request":
		b := bytes.Buffer{}
		p.printRequest(&b, command.Template, command.DefaultData)

		if err := p.XMLValidator.ValidateXML(b.Bytes()); err != nil {
			p.Cli.Println(err.Error())
		} else {
			p.Cli.Println("ok")
		}

		// see if the user want to update some more data
		return p.setupCommandData(command)
	}

	// choice will contain the name + the current value, split to get the name
	name := strings.Split(choice, " ")[0]
	currentValue := data[name]
	p.updateValue(currentValue, command.DefaultData)

	// see if the user want to update some more data
	return p.setupCommandData(command)
}

// updateValue handles different types of values to update.
func (p *Prompt) updateValue(currentValue *FieldValue, parent any) {
	switch currentValue.Kind {
	case reflect.Int:
		p.updateInt(currentValue, parent)

	case reflect.Slice:
		switch currentValue.ListKind {
		case reflect.Slice, reflect.Struct, reflect.Ptr:
			p.updateListOfStructs(currentValue, parent)

		default:
			p.updateListValue(currentValue, parent)
		}

	case reflect.String:
		p.updateString(currentValue, parent)

	case reflect.Struct:
		p.updateStruct(currentValue, parent)

	case reflect.Ptr:
		// need to dereference to be able to handle pointers to simple values
		p.updateValue(DereferencePointer(parent, currentValue), parent)

	case reflect.Bool:
		p.updateBool(currentValue, parent)

	default:
		panic(fmt.Sprintf("currently unsupported kind %v", currentValue.Kind))
	}
}

// updateBool set the bool value to the opposite of what it currently is.
func (*Prompt) updateBool(currentValue *FieldValue, parent any) {
	SetValue(parent, currentValue.Name, !currentValue.Value.Bool())
}

// updateStruct present the current struct values to the user and ask them to update
// what they need to update before sending the request to the server.
func (p *Prompt) updateStruct(currentValue *FieldValue, parent any) {
	// can't set values on a nil struct
	InitializeIfNil(parent, currentValue)
	curr := GetFromTag(parent, currentValue.Name)

	for {
		options := []string{}
		data := map[string]*FieldValue{}

		for _, v := range GetFieldValues(curr) {
			// show current request data
			options = append(options, fmt.Sprintf("%s (%s): %s", v.Name, v.Type, v.ValueAsString()))
			data[v.Name] = v
		}

		options = append(options, done)

		pu := promptui.Select{
			Label: "Current data",
			Items: options,
			Size:  15,
		}

		_, choice, err := pu.Run()
		if err != nil {
			panic(err)
		}

		if choice == done {
			return
		}

		// choice will contain the name + the current value, split to get the name
		name := strings.Split(choice, " ")[0]
		currentValue = data[name]
		p.updateValue(currentValue, curr)
	}
}

// updateInt update an int value.
func (*Prompt) updateInt(currentValue *FieldValue, parent any) {
	defaultVal := strconv.Itoa(int(currentValue.Value.Int()))

	v := promptui.Prompt{
		Label:     "New value",
		Default:   defaultVal,
		AllowEdit: true,
		Validate: func(s string) error {
			// don't allow anything but an int
			_, err := strconv.Atoi(s)
			if err != nil {
				return errors.New("not an int")
			}

			return nil
		},
	}

	newValue, err := v.Run()
	if err != nil {
		panic(err)
	}

	// no need to check error here since the validation function of promptui checks it
	i, _ := strconv.Atoi(newValue)

	SetValue(parent, currentValue.Name, i)
}

// updateString update a string value.
func (*Prompt) updateString(currentValue *FieldValue, parent any) {
	v := promptui.Prompt{
		Label:     "New value",
		Default:   currentValue.Value.String(),
		AllowEdit: true,
	}

	newValue, err := v.Run()
	if err != nil {
		panic(err)
	}

	SetValue(parent, currentValue.Name, newValue)
}

// updateListValue add or remove values from list of simple values.
func (*Prompt) updateListValue(currentValue *FieldValue, parent any) {
	curr := currentValue.ListValuesAsString()

	for {
		options := curr
		options = append(options, done)

		pu := promptui.SelectWithAdd{
			Label:    "Select item to remove, or add new",
			Items:    options,
			AddLabel: "Add",
		}

		// returns idx == -1 if new value was added
		idx, newValue, err := pu.Run()
		if err != nil {
			panic(err)
		}

		if newValue == done {
			break
		}

		ret := []string{}

		if idx >= 0 {
			// remove the chosen value
			for i, v := range curr {
				if idx == i {
					continue
				}

				ret = append(ret, v)
			}
		} else {
			// add the new value
			ret = append(ret, curr...)
			ret = append(ret, newValue)
		}

		curr = ret
	}

	// set the new value
	SetValue(parent, currentValue.Name, curr)
}

// updateListOfStructs add, update or remove structs in the list.
func (p *Prompt) updateListOfStructs(currentValue *FieldValue, parent any) {
	for {
		// get the list to be able to update it
		curr := GetFromTag(parent, currentValue.Name)
		options := []string{"Add"}
		data := map[string]*FieldValue{}

		for _, v := range GetFieldValues(curr) {
			// show the current request data
			val := v.ValueAsString()
			options = append(options, val)
			data[val] = v
		}

		options = append(options, done)

		pu := promptui.Select{
			Label: "Select item to remove/update, or add new",
			Items: options,
			Size:  15,
		}

		idx, newValue, err := pu.Run()
		if err != nil {
			panic(err)
		}

		switch newValue {
		case done:
			// done editing
			SetValue(parent, currentValue.Name, curr)
			return

		case "Add":
			p.addNewStructToList(currentValue)
			continue
		}

		pu = promptui.Select{
			Label: "What do you want to do?",
			Items: []string{"Update", "Delete"},
		}

		_, action, err := pu.Run()
		if err != nil {
			panic(err)
		}

		if action == "Update" {
			choice := data[newValue]

			if choice.Kind == reflect.Struct || choice.Kind == reflect.Pointer {
				// idx - 1 because "Add" is first in the options list
				p.updateStructInList(idx-1, curr)
				continue
			}

			p.updateValue(choice, curr)

			continue
		}

		// delete was chosen
		// idx - 1 because "Add" is first in the options list
		currentValue.RemoveAtIndex(idx - 1)
	}
}

// addNewStructToList create a new instance of the struct type and update the values in it before
// adding to the list.
func (p *Prompt) addNewStructToList(currentValue *FieldValue) {
	// create new instance of the list struct type to add to the list
	newObj := currentValue.NewOfListType()

	for {
		newOptions := []string{}
		newData := map[string]*FieldValue{}

		for _, v := range GetFieldValues(newObj) {
			// show current request data
			newOptions = append(newOptions, fmt.Sprintf("%s (%s): %s", v.Name, v.Type, v.ValueAsString()))
			newData[v.Name] = v
		}

		newOptions = append(newOptions, "Add")

		pu := promptui.Select{
			Label: "Select Data",
			Items: newOptions,
			Size:  15,
		}

		_, choice, err := pu.Run()
		if err != nil {
			panic(err)
		}

		if choice == "Add" {
			// done editing
			currentValue.AddToStructList(newObj)

			break
		}

		// choice will contain the name + the current value, split to get the name
		name := strings.Split(choice, " ")[0]
		p.updateValue(newData[name], newObj)
	}
}

// updateStructInList ask the user to update the wanted values of the object.
func (p *Prompt) updateStructInList(idx int, parent any) {
	for {
		// get the object to update from the list
		curr := GetFieldFromList(idx, parent)
		options := []string{}
		data := map[string]*FieldValue{}

		for _, v := range GetFieldValues(curr) {
			// show current request data
			options = append(options, fmt.Sprintf("%s (%s): %s", v.Name, v.Type, v.ValueAsString()))
			data[v.Name] = v
		}

		options = append(options, done)

		pu := promptui.Select{
			Label: "Current data",
			Items: options,
			Size:  15,
		}

		_, choice, err := pu.Run()
		if err != nil {
			panic(err)
		}

		if choice == done {
			return
		}

		// choice will contain the name + the current value, split to get the name
		name := strings.Split(choice, " ")[0]
		currentValue := data[name]
		p.updateValue(currentValue, curr)
	}
}
