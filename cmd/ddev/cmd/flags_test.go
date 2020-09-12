package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	asrt "github.com/stretchr/testify/assert"
)

const (
	commandName = "command"
	script      = "script"
)

func getSubject() Flags {
	var subject Flags
	subject.Init(commandName, script)
	return subject
}

func TestUnitCmdFlagsInit(t *testing.T) {
	assert := asrt.New(t)
	subject := getSubject()

	assert.Exactly(commandName, subject.CommandName)
	assert.Exactly(script, subject.Script)
}

// TestCmdFlagsLoadFromJSON checks LoadFromJSON works correctly and handles
// user errors.
func TestUnitCmdFlagsLoadFromJSON(t *testing.T) {
	assert := asrt.New(t)
	subject := getSubject()

	// No data
	assert.NoError(subject.LoadFromJSON(``))

	// Invalid JSON
	assert.Error(subject.LoadFromJSON(`this is no valid JSON`))

	// Empty array
	assert.NoError(subject.LoadFromJSON(`[]`))

	// Minimal
	assert.NoError(subject.LoadFromJSON(`[{"Name":"test","Usage":"Usage of test"}]`))
	assert.EqualValues("test", subject.Definition[0].Name)
	assert.EqualValues("", subject.Definition[0].Shorthand)
	assert.EqualValues("Usage of test", subject.Definition[0].Usage)
	assert.EqualValues("bool", subject.Definition[0].Type)
	assert.EqualValues("false", subject.Definition[0].DefValue)
	assert.EqualValues("", subject.Definition[0].NoOptDefVal)
	assert.Empty(subject.Definition[0].Annotations)

	// Full
	assert.NoError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":"Usage of test 1","Type":"string","DefValue":"true","NoOptDefVal":"true","Annotations":{"test-1":["test-1-1","test-1-2"]}},{"Name":"test-2","Usage":"Usage of test 2","Type":"bool","DefValue":"true","NoOptDefVal":"true","Annotations":{"test-2":["test-2-1","test-2-2"]}}]`))
	assert.EqualValues("test-1", subject.Definition[0].Name)
	assert.EqualValues("t", subject.Definition[0].Shorthand)
	assert.EqualValues("Usage of test 1", subject.Definition[0].Usage)
	assert.EqualValues("string", subject.Definition[0].Type)
	assert.EqualValues("true", subject.Definition[0].DefValue)
	assert.EqualValues("true", subject.Definition[0].NoOptDefVal)
	assert.EqualValues(map[string][]string{"test-1": {"test-1-1", "test-1-2"}}, subject.Definition[0].Annotations)
	assert.EqualValues("test-2", subject.Definition[1].Name)
	assert.EqualValues("", subject.Definition[1].Shorthand)
	assert.EqualValues("Usage of test 2", subject.Definition[1].Usage)
	assert.EqualValues("bool", subject.Definition[1].Type)
	assert.EqualValues("true", subject.Definition[1].DefValue)
	assert.EqualValues("true", subject.Definition[1].NoOptDefVal)
	assert.EqualValues(map[string][]string{"test-2": {"test-2-1", "test-2-2"}}, subject.Definition[1].Annotations)

	// Duplicate flag
	assert.EqualError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":"Usage of test 1"},{"Name":"test-1","Usage":"Test duplicate"}]`),
		"The following problems were found in the flags definition of the command 'command' in 'script':\n * for flag 'test-1':\n   - flag 'test-1' already defined")

	// Duplicate shorthand
	assert.EqualError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":"Usage of test 1"},{"Name":"test-2","Shorthand":"t","Usage":"Usage of test 2 with existing shorthand"}]`),
		"The following problems were found in the flags definition of the command 'command' in 'script':\n * for flag 'test-2':\n   - shorthand 't' is already defined for flag 'test-1'")

	// Invalid shorthand
	assert.EqualError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t1","Usage":"Usage of test 1"}]`),
		"The following problems were found in the flags definition of the command 'command' in 'script':\n * for flag 'test-1':\n   - shorthand 't1' is more than one ASCII character")

	// Empty usage in multiple commands
	assert.EqualError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":""},{"Name":"test-2"}]`),
		"The following problems were found in the flags definition of the command 'command' in 'script':\n * for flag 'test-1':\n   - no usage defined\n * for flag 'test-2':\n   - no usage defined")

	// Invalid and not implemented type
	assert.EqualError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":"Usage of test 1","Type":"invalid"},{"Name":"test-2","Usage":"Usage of test 2","Type":"notimplemented"}]`),
		"The following problems were found in the flags definition of the command 'command' in 'script':\n * for flag 'test-1':\n   - type 'invalid' is not known\n * for flag 'test-2':\n   - type 'notimplemented' is not implemented")
}

func getCommand() cobra.Command {
	return cobra.Command{
		Use:     "usage of command",
		Short:   "short description",
		Example: "example",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

}

// TestUnitCmdFlagsAssignToCommand checks AssignToCommand works correctly and
// handles user errors.
func TestUnitCmdFlagsAssignToCommand(t *testing.T) {
	assert := asrt.New(t)
	subject := getSubject()

	var c cobra.Command

	// No flags
	c = getCommand()
	assert.NoError(subject.AssignToCommand(&c))

	// Minimal
	assert.NoError(subject.LoadFromJSON(`[{"Name":"test","Usage":"Usage of test"}]`))
	assert.NoError(subject.AssignToCommand(&c))
	assert.EqualValues("test", subject.Definition[0].Name)
	assert.EqualValues("", subject.Definition[0].Shorthand)
	assert.EqualValues("Usage of test", subject.Definition[0].Usage)
	assert.EqualValues("bool", subject.Definition[0].Type)
	assert.EqualValues("false", subject.Definition[0].DefValue)
	assert.EqualValues("", subject.Definition[0].NoOptDefVal)
	assert.Empty(subject.Definition[0].Annotations)

	// Full
	c = getCommand()
	assert.NoError(subject.LoadFromJSON(`[{"Name":"test-1","Shorthand":"t","Usage":"Usage of test 1","Type":"string","DefValue":"true","NoOptDefVal":"true","Annotations":{"test-1":["test-1-1","test-1-2"]}},{"Name":"test-2","Usage":"Usage of test 2","Type":"bool","DefValue":"true","NoOptDefVal":"true","Annotations":{"test-2":["test-2-1","test-2-2"]}}]`))
	assert.NoError(subject.AssignToCommand(&c))
	assert.EqualValues("test-1", subject.Definition[0].Name)
	assert.EqualValues("t", subject.Definition[0].Shorthand)
	assert.EqualValues("Usage of test 1", subject.Definition[0].Usage)
	assert.EqualValues("string", subject.Definition[0].Type)
	assert.EqualValues("true", subject.Definition[0].DefValue)
	assert.EqualValues("true", subject.Definition[0].NoOptDefVal)
	assert.EqualValues(map[string][]string{"test-1": {"test-1-1", "test-1-2"}}, subject.Definition[0].Annotations)
	assert.EqualValues("test-2", subject.Definition[1].Name)
	assert.EqualValues("", subject.Definition[1].Shorthand)
	assert.EqualValues("Usage of test 2", subject.Definition[1].Usage)
	assert.EqualValues("bool", subject.Definition[1].Type)
	assert.EqualValues("true", subject.Definition[1].DefValue)
	assert.EqualValues("true", subject.Definition[1].NoOptDefVal)
	assert.EqualValues(map[string][]string{"test-2": {"test-2-1", "test-2-2"}}, subject.Definition[1].Annotations)
}
