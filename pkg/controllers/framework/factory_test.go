/*
Copyright 2026 zhaizhicheng.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"testing"
)

type mockController struct {
	name string
}

func (m *mockController) Name() string {
	return m.name
}

func (m *mockController) Initialize(opt *ControllerOption) error {
	return nil
}

func (m *mockController) Run(stopCh <-chan struct{}) {}

func TestRegisterController(t *testing.T) {
	// Reset controllers map for testing
	originalControllers := controllers
	controllers = map[string]Controller{}
	defer func() { controllers = originalControllers }()

	t.Run("Register nil controller", func(t *testing.T) {
		err := RegisterController(nil)
		if err == nil {
			t.Error("expected error when registering nil controller")
		}
	})

	t.Run("Register valid controller", func(t *testing.T) {
		name := "test-controller"
		ctrl := &mockController{name: name}
		err := RegisterController(ctrl)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if _, found := controllers[name]; !found {
			t.Errorf("controller %s not found in registry", name)
		}
	})

	t.Run("Register duplicate controller", func(t *testing.T) {
		name := "dup-controller"
		ctrl := &mockController{name: name}
		_ = RegisterController(ctrl)
		err := RegisterController(ctrl)
		if err == nil {
			t.Error("expected error when registering duplicate controller")
		}
	})
}

func TestForeachController(t *testing.T) {
	// Reset controllers map for testing
	originalControllers := controllers
	controllers = map[string]Controller{}
	defer func() { controllers = originalControllers }()

	ctrl1 := &mockController{name: "ctrl1"}
	ctrl2 := &mockController{name: "ctrl2"}
	_ = RegisterController(ctrl1)
	_ = RegisterController(ctrl2)

	visited := make(map[string]bool)
	ForeachController(func(c Controller) {
		visited[c.Name()] = true
	})

	if len(visited) != 2 {
		t.Errorf("expected 2 visited controllers, got %d", len(visited))
	}
	if !visited["ctrl1"] || !visited["ctrl2"] {
		t.Errorf("missing visited controllers: %v", visited)
	}
}
