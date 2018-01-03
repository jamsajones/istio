// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model_test

import (
	"reflect"
	"testing"

	proxyconfig "istio.io/api/proxy/v1/config"
	"istio.io/istio/pilot/model"
)

func TestRejectConflictingEgressRules(t *testing.T) {
	cases := []struct {
		name  string
		in    map[string]*proxyconfig.EgressRule
		out   map[string]*proxyconfig.EgressRule
		valid bool
	}{
		{name: "no conflicts",
			in: map[string]*proxyconfig.EgressRule{"cnn": {
				Destination: &proxyconfig.IstioService{
					Service: "*cnn.com",
				},
				Ports: []*proxyconfig.EgressRule_Port{
					{Port: 80, Protocol: "http"},
					{Port: 443, Protocol: "https"},
				},
			},
				"bbc": {
					Destination: &proxyconfig.IstioService{
						Service: "*bbc.com",
					},

					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			out: map[string]*proxyconfig.EgressRule{"cnn": {
				Destination: &proxyconfig.IstioService{
					Service: "*cnn.com",
				},
				Ports: []*proxyconfig.EgressRule_Port{
					{Port: 80, Protocol: "http"},
					{Port: 443, Protocol: "https"},
				},
			},
				"bbc": {
					Destination: &proxyconfig.IstioService{
						Service: "*bbc.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			valid: true},
		{name: "a conflict in a domain",
			in: map[string]*proxyconfig.EgressRule{"cnn2": {
				Destination: &proxyconfig.IstioService{
					Service: "*cnn.com",
				},
				Ports: []*proxyconfig.EgressRule_Port{
					{Port: 80, Protocol: "http"},
					{Port: 443, Protocol: "https"},
				},
			},
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			out: map[string]*proxyconfig.EgressRule{
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			valid: false},
		{name: "a conflict in a domain, different ports",
			in: map[string]*proxyconfig.EgressRule{"cnn2": {
				Destination: &proxyconfig.IstioService{
					Service: "*cnn.com",
				},
				Ports: []*proxyconfig.EgressRule_Port{
					{Port: 80, Protocol: "http"},
					{Port: 443, Protocol: "https"},
				},
			},
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 8080, Protocol: "http"},
						{Port: 8081, Protocol: "https"},
					},
				},
			},
			out: map[string]*proxyconfig.EgressRule{
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 8080, Protocol: "http"},
						{Port: 8081, Protocol: "https"},
					},
				},
			},
			valid: false},
		{name: "two conflicts, two rules rejected",
			in: map[string]*proxyconfig.EgressRule{"cnn2": {
				Destination: &proxyconfig.IstioService{
					Service: "*cnn.com",
				},
				Ports: []*proxyconfig.EgressRule_Port{
					{Port: 80, Protocol: "http"},
					{Port: 443, Protocol: "https"},
				},
			},
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
				"cnn3": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			out: map[string]*proxyconfig.EgressRule{
				"cnn1": {
					Destination: &proxyconfig.IstioService{
						Service: "*cnn.com",
					},
					Ports: []*proxyconfig.EgressRule_Port{
						{Port: 80, Protocol: "http"},
						{Port: 443, Protocol: "https"},
					},
				},
			},
			valid: false},
	}

	for _, c := range cases {
		got, errs := model.RejectConflictingEgressRules(c.in)
		if (errs == nil) != c.valid {
			t.Errorf("RejectConflictingEgressRules failed on %s: got valid=%v but wanted valid=%v",
				c.name, errs == nil, c.valid)
		}
		if !reflect.DeepEqual(got, c.out) {
			t.Errorf("RejectConflictingEgressRules failed on %s: got=%v but wanted %v: %v",
				c.name, got, c.in)
		}
	}
}
