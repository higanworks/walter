/* walter: a deployment pipeline template
 * Copyright (C) 2014 Recruit Technologies Co., Ltd. and contributors
 * (see CONTRIBUTORS.md)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package engine

import (
	"testing"

	"github.com/recruit-tech/walter/config"
	"github.com/recruit-tech/walter/messengers"
	"github.com/recruit-tech/walter/pipelines"
	"github.com/recruit-tech/walter/stages"
	"github.com/stretchr/testify/assert"
)

func createShellScriptStage(name string, fileName string) *stages.ShellScriptStage {
	in := make(chan stages.Mediator)
	out := make(chan stages.Mediator)
	return &stages.ShellScriptStage{
		CommandStage: stages.CommandStage{
			BaseStage: stages.BaseStage{
				StageName: name,
				InputCh:   &in,
				OutputCh:  &out,
			},
		},
		File: fileName,
	}
}

func createCommandStageWithOnlyIf(name string, command string, only_if string) *stages.CommandStage {
	in := make(chan stages.Mediator)
	out := make(chan stages.Mediator)
	return &stages.CommandStage{
		Command: command,
		OnlyIf:  only_if,
		BaseStage: stages.BaseStage{
			StageName: name,
			InputCh:   &in,
			OutputCh:  &out,
		},
	}
}

func createCommandStageWithName(name string, command string) *stages.CommandStage {
	return createCommandStageWithOnlyIf(name, command, "")
}

func createCommandStage(command string) *stages.CommandStage {
	return createCommandStageWithName(command, command)
}

func execute(stage stages.Stage) stages.Mediator {
	mon := make(chan stages.Mediator)
	e := &Engine{
		MonitorCh: &mon,
		Pipeline: &pipelines.Pipeline{
			Reporter: &messengers.FakeMessenger{},
		},
	}

	go e.ExecuteStage(stage)

	mediator := stages.Mediator{States: make(map[string]string), Type: "start"}
	go func() {
		*stage.GetInputCh() <- mediator
		close(*stage.GetInputCh())
	}()

	for {
		_, ok := <-*stage.GetOutputCh()
		if !ok {
			break
		}
	}

	var m stages.Mediator
	acm := stages.Mediator{States: make(map[string]string)}
	for {
		m = <-mon
		for k, v := range m.States {
			acm.States[k] = v
		}
		if m.Type == "end" {
			break
		}
	}
	return acm
}

func TestRunOnce(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createCommandStage("echo foobar"))
	pipeline.AddStage(createCommandStage("echo baz"))
	monitorCh := make(chan stages.Mediator)
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
	}
	m := engine.RunOnce()

	assert.Equal(t, "true", m.States["echo foobar"])
	assert.Equal(t, false, m.IsAnyFailure())
}

func TestRunOnceWithShellScriptStage(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createShellScriptStage("foobar-shell", "../stages/test_sample.sh"))
	monitorCh := make(chan stages.Mediator)
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
	}
	m := engine.RunOnce()

	assert.Equal(t, "true", m.States["foobar-shell"])
	assert.Equal(t, false, m.IsAnyFailure())
}

func TestRunOnceWithOptsOffStopOnAnyFailure(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createCommandStage("echo foobar"))
	pipeline.AddStage(createCommandStage("thisiserrorcommand"))
	pipeline.AddStage(createCommandStage("echo foobar2"))
	monitorCh := make(chan stages.Mediator)
	o := &config.Opts{StopOnAnyFailure: false}
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
		Opts:      o,
	}
	m := engine.RunOnce()

	assert.Equal(t, "false", m.States["echo foobar2"])
	assert.Equal(t, true, m.IsAnyFailure())
}

func TestRunOnceWithOptsOnStopOnAnyFailure(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createCommandStage("echo foobar"))
	pipeline.AddStage(createCommandStage("thisiserrorcommand"))
	pipeline.AddStage(createCommandStage("echo foobar2"))
	monitorCh := make(chan stages.Mediator)
	o := &config.Opts{StopOnAnyFailure: true}
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
		Opts:      o,
	}

	m := engine.RunOnce()

	assert.Equal(t, "true", m.States["echo foobar2"])
	assert.Equal(t, true, m.IsAnyFailure())
}

func TestRunOnceWithOnlyIfFailure(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createCommandStageWithOnlyIf("first", "echo first", "test 1 -lt 1"))
	pipeline.AddStage(createCommandStageWithName("second", "echo second"))
	pipeline.AddStage(createCommandStageWithName("third", "echo third"))
	monitorCh := make(chan stages.Mediator)
	o := &config.Opts{}
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
		Opts:      o,
	}
	m := engine.RunOnce()

	assert.Equal(t, 3, len(m.States))
	assert.Equal(t, "true", m.States["first"])
	assert.Equal(t, "true", m.States["second"])
	assert.Equal(t, "true", m.States["third"])
	assert.Equal(t, false, m.IsAnyFailure())
}

func TestRunOnceWithOnlyIfSuccess(t *testing.T) {
	pipeline := &pipelines.Pipeline{
		Reporter: &messengers.FakeMessenger{},
	}
	pipeline.AddStage(createCommandStageWithOnlyIf("first", "echo first", "test 1 -eq 1"))
	pipeline.AddStage(createCommandStageWithName("second", "echo second"))
	pipeline.AddStage(createCommandStageWithName("third", "echo third"))
	monitorCh := make(chan stages.Mediator)
	o := &config.Opts{}
	engine := &Engine{
		Pipeline:  pipeline,
		MonitorCh: &monitorCh,
		Opts:      o,
	}
	m := engine.RunOnce()

	assert.Equal(t, 3, len(m.States))
	assert.Equal(t, "true", m.States["first"])
	assert.Equal(t, "true", m.States["second"])
	assert.Equal(t, "true", m.States["third"])
	assert.Equal(t, false, m.IsAnyFailure())
}

func TestExecuteWithSingleStage(t *testing.T) {
	stage := createCommandStageWithName("test_command_stage", "ls")
	actual := execute(stage)
	assert.Equal(t, "true", actual.States[stage.StageName])
}

func TestExecuteWithSingleStageFailed(t *testing.T) {
	stage := createCommandStageWithName("test_command_stage", "nothingcommand")
	actual := execute(stage)
	assert.Equal(t, "false", actual.States[stage.StageName])
}

func TestExecuteWithSingleStageHasChild(t *testing.T) {
	stage := createCommandStageWithName("test_command_stage", "ls -l")
	child := createCommandStageWithName("test_child", "ls -l")
	stage.AddChildStage(child)
	actual := execute(stage)
	assert.Equal(t, "true", actual.States[stage.StageName])
}

func TestExecuteWithSingleStageHasErrChild(t *testing.T) {
	stage := createCommandStageWithName("test_command_stage", "ls -l")
	child := createCommandStageWithName("test_child", "nothingcommand")
	stage.AddChildStage(child)
	acm := execute(stage)

	t.Logf("accumulated output: %+v", acm)
	assert.Equal(t, "true", acm.States[stage.StageName])
	assert.Equal(t, "false", acm.States[child.StageName])
}
