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

package validate

import (
	"fmt"
)

type Vertex struct {
	Edges []*Vertex
	Name  string
}

func NewVertex(name string) *Vertex { return &Vertex{Name: name, Edges: []*Vertex{}} }

func LoadVertexs(data map[string][]string) ([]*Vertex, error) {
	graphMap := make(map[string]*Vertex, len(data))
	output := make([]*Vertex, 0, len(data))

	for name := range data {
		graphMap[name] = NewVertex(name)
	}

	for name, values := range data {
		graphMap[name].Edges = make([]*Vertex, len(values))
		for index, value := range values {
			// Check for self-loop
			if value == name {
				return nil, fmt.Errorf("flow '%s' has self-dependency", name)
			}

			if _, ok := graphMap[value]; !ok {
				return nil, fmt.Errorf("%s: %s", VertexNotDefinedError.Error(), value)
			}
			graphMap[name].Edges[index] = graphMap[value]
		}
	}

	for _, value := range graphMap {
		output = append(output, value)
	}
	return output, nil
}

// IsDAG uses three-color DFS to detect cycles in O(V+E) time and O(V) space.
// Colors: 0=white (unvisited), 1=gray (visiting), 2=black (visited)
func IsDAG(graph []*Vertex) bool {
	colors := make(map[*Vertex]int, len(graph))

	for _, vertex := range graph {
		if colors[vertex] == 0 {
			if hasCycle(vertex, colors) {
				return false
			}
		}
	}
	return true
}

// hasCycle performs DFS with three-color marking to detect back edges (cycles).
func hasCycle(vertex *Vertex, colors map[*Vertex]int) bool {
	colors[vertex] = 1 // Mark as visiting (gray)

	for _, neighbor := range vertex.Edges {
		if colors[neighbor] == 1 {
			// Back edge detected - cycle exists
			return true
		}
		if colors[neighbor] == 0 && hasCycle(neighbor, colors) {
			return true
		}
	}

	colors[vertex] = 2 // Mark as visited (black)
	return false
}
