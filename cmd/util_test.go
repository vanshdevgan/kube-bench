// Copyright © 2017 Aqua Security Software Ltd. <info@aquasec.com>
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

package cmd

import (
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/spf13/viper"
)

func TestCheckVersion(t *testing.T) {
	kubeoutput := `Client Version: version.Info{Major:"1", Minor:"7", GitVersion:"v1.7.0", GitCommit:"d3ada0119e776222f11ec7945e6d860061339aad", GitTreeState:"clean", BuildDate:"2017-06-30T09:51:01Z", GoVersion:"go1.8.3", Compiler:"gc", Platform:"darwin/amd64"}
	Server Version: version.Info{Major:"1", Minor:"7", GitVersion:"v1.7.0", GitCommit:"d3ada0119e776222f11ec7945e6d860061339aad", GitTreeState:"clean", BuildDate:"2017-07-26T00:12:31Z", GoVersion:"go1.8.3", Compiler:"gc", Platform:"linux/amd64"}`
	cases := []struct {
		t     string
		s     string
		major string
		minor string
		exp   string
	}{
		{t: "Client", s: kubeoutput, major: "1", minor: "7"},
		{t: "Server", s: kubeoutput, major: "1", minor: "7"},
		{t: "Client", s: kubeoutput, major: "1", minor: "6", exp: "Unexpected Client version 1.7"},
		{t: "Client", s: kubeoutput, major: "2", minor: "0", exp: "Unexpected Client version 1.7"},
		{t: "Server", s: "something unexpected", major: "2", minor: "0", exp: "Couldn't find Server version from kubectl output 'something unexpected'"},
	}

	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			m := checkVersion(c.t, c.s, c.major, c.minor)
			if m != c.exp {
				t.Fatalf("Got: %s, expected: %s", m, c.exp)
			}
		})
	}

}

func TestVersionMatch(t *testing.T) {
	minor := regexVersionMinor
	major := regexVersionMajor
	client := `Client Version: version.Info{Major:"1", Minor:"7", GitVersion:"v1.7.0", GitCommit:"d3ada0119e776222f11ec7945e6d860061339aad", GitTreeState:"clean", BuildDate:"2017-06-30T09:51:01Z", GoVersion:"go1.8.3", Compiler:"gc", Platform:"darwin/amd64"}`
	server := `Server Version: version.Info{Major:"1", Minor:"7", GitVersion:"v1.7.0", GitCommit:"d3ada0119e776222f11ec7945e6d860061339aad", GitTreeState:"clean", BuildDate:"2017-07-26T00:12:31Z", GoVersion:"go1.8.3", Compiler:"gc", Platform:"linux/amd64"}`

	cases := []struct {
		r   *regexp.Regexp
		s   string
		exp string
	}{
		{r: major, s: server, exp: "1"},
		{r: minor, s: server, exp: "7"},
		{r: major, s: client, exp: "1"},
		{r: minor, s: client, exp: "7"},
		{r: major, s: "Some unexpected string"},
		{r: minor}, // Checking that we don't fall over if the string is empty
	}

	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			m := versionMatch(c.r, c.s)
			if m != c.exp {
				t.Fatalf("Got %s expected %s", m, c.exp)
			}
		})
	}
}

var g string
var e []error
var eIndex int

func fakeps(proc string) string {
	return g
}

func fakestat(file string) (os.FileInfo, error) {
	err := e[eIndex]
	eIndex++
	return nil, err
}

func TestVerifyBin(t *testing.T) {
	cases := []struct {
		proc  string
		psOut string
		exp   bool
	}{
		{proc: "single", psOut: "single", exp: true},
		{proc: "single", psOut: "", exp: false},
		{proc: "two words", psOut: "two words", exp: true},
		{proc: "two words", psOut: "", exp: false},
		{proc: "cmd", psOut: "cmd param1 param2", exp: true},
		{proc: "cmd param", psOut: "cmd param1 param2", exp: true},
		{proc: "cmd param", psOut: "cmd", exp: false},
		{proc: "cmd", psOut: "cmd x \ncmd y", exp: true},
		{proc: "cmd y", psOut: "cmd x \ncmd y", exp: true},
		{proc: "cmd", psOut: "/usr/bin/cmd", exp: true},
		{proc: "cmd", psOut: "kube-cmd", exp: false},
		{proc: "cmd", psOut: "/usr/bin/kube-cmd", exp: false},
	}

	psFunc = fakeps
	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			g = c.psOut
			v := verifyBin(c.proc)
			if v != c.exp {
				t.Fatalf("Expected %v got %v", c.exp, v)
			}
		})
	}
}

func TestFindExecutable(t *testing.T) {
	cases := []struct {
		candidates []string // list of executables we'd consider
		psOut      string   // fake output from ps
		exp        string   // the one we expect to find in the (fake) ps output
		expErr     bool
	}{
		{candidates: []string{"one", "two", "three"}, psOut: "two", exp: "two"},
		{candidates: []string{"one", "two", "three"}, psOut: "two three", exp: "two"},
		{candidates: []string{"one double", "two double", "three double"}, psOut: "two double is running", exp: "two double"},
		{candidates: []string{"one", "two", "three"}, psOut: "blah", expErr: true},
		{candidates: []string{"one double", "two double", "three double"}, psOut: "two", expErr: true},
		{candidates: []string{"apiserver", "kube-apiserver"}, psOut: "kube-apiserver", exp: "kube-apiserver"},
		{candidates: []string{"apiserver", "kube-apiserver", "hyperkube-apiserver"}, psOut: "kube-apiserver", exp: "kube-apiserver"},
	}

	psFunc = fakeps
	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			g = c.psOut
			e, err := findExecutable(c.candidates)
			if e != c.exp {
				t.Fatalf("Expected %v got %v", c.exp, e)
			}

			if err == nil && c.expErr {
				t.Fatalf("Expected error")
			}

			if err != nil && !c.expErr {
				t.Fatalf("Didn't expect error: %v", err)
			}
		})
	}
}

func TestGetBinaries(t *testing.T) {
	cases := []struct {
		config map[string]interface{}
		psOut  string
		exp    map[string]string
	}{
		{
			config: map[string]interface{}{"components": []string{"apiserver"}, "apiserver": map[string]interface{}{"bins": []string{"apiserver", "kube-apiserver"}}},
			psOut:  "kube-apiserver",
			exp:    map[string]string{"apiserver": "kube-apiserver"},
		},
		{
			// "thing" is not in the list of components
			config: map[string]interface{}{"components": []string{"apiserver"}, "apiserver": map[string]interface{}{"bins": []string{"apiserver", "kube-apiserver"}}, "thing": map[string]interface{}{"bins": []string{"something else", "thing"}}},
			psOut:  "kube-apiserver thing",
			exp:    map[string]string{"apiserver": "kube-apiserver"},
		},
		{
			// "anotherthing" in list of components but doesn't have a defintion
			config: map[string]interface{}{"components": []string{"apiserver", "anotherthing"}, "apiserver": map[string]interface{}{"bins": []string{"apiserver", "kube-apiserver"}}, "thing": map[string]interface{}{"bins": []string{"something else", "thing"}}},
			psOut:  "kube-apiserver thing",
			exp:    map[string]string{"apiserver": "kube-apiserver"},
		},
		{
			// more than one component
			config: map[string]interface{}{"components": []string{"apiserver", "thing"}, "apiserver": map[string]interface{}{"bins": []string{"apiserver", "kube-apiserver"}}, "thing": map[string]interface{}{"bins": []string{"something else", "thing"}}},
			psOut:  "kube-apiserver \nthing",
			exp:    map[string]string{"apiserver": "kube-apiserver", "thing": "thing"},
		},
		{
			// default binary to component name
			config: map[string]interface{}{"components": []string{"apiserver", "thing"}, "apiserver": map[string]interface{}{"bins": []string{"apiserver", "kube-apiserver"}}, "thing": map[string]interface{}{"bins": []string{"something else", "thing"}, "optional": true}},
			psOut:  "kube-apiserver \notherthing some params",
			exp:    map[string]string{"apiserver": "kube-apiserver", "thing": "thing"},
		},
	}

	v := viper.New()
	psFunc = fakeps

	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			g = c.psOut
			for k, val := range c.config {
				v.Set(k, val)
			}
			m := getBinaries(v)
			if !reflect.DeepEqual(m, c.exp) {
				t.Fatalf("Got %v\nExpected %v", m, c.exp)
			}
		})
	}
}

func TestMultiWordReplace(t *testing.T) {
	cases := []struct {
		input   string
		sub     string
		subname string
		output  string
	}{
		{input: "Here's a file with no substitutions", sub: "blah", subname: "blah", output: "Here's a file with no substitutions"},
		{input: "Here's a file with a substitution", sub: "blah", subname: "substitution", output: "Here's a file with a blah"},
		{input: "Here's a file with multi-word substitutions", sub: "multi word", subname: "multi-word", output: "Here's a file with 'multi word' substitutions"},
		{input: "Here's a file with several several substitutions several", sub: "blah", subname: "several", output: "Here's a file with blah blah substitutions blah"},
	}
	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			s := multiWordReplace(c.input, c.subname, c.sub)
			if s != c.output {
				t.Fatalf("Expected %s got %s", c.output, s)
			}
		})
	}
}

func TestGetKubeVersion(t *testing.T) {
	ver := getKubeVersion()
	if ver == nil {
		t.Log("Expected non nil version info.")
	} else {
		if ok, err := regexp.MatchString(`\d+.\d+`, ver.Client); !ok && err != nil {
			t.Logf("Expected:%v got %v\n", "n.m", ver.Client)
		}

		if ok, err := regexp.MatchString(`\d+.\d+`, ver.Server); !ok && err != nil {
			t.Logf("Expected:%v got %v\n", "n.m", ver.Server)
		}

	}
}

func TestFindConfigFile(t *testing.T) {
	cases := []struct {
		input       []string
		statResults []error
		exp         string
	}{
		{input: []string{"myfile"}, statResults: []error{nil}, exp: "myfile"},
		{input: []string{"thisfile", "thatfile"}, statResults: []error{os.ErrNotExist, nil}, exp: "thatfile"},
		{input: []string{"thisfile", "thatfile"}, statResults: []error{os.ErrNotExist, os.ErrNotExist}, exp: ""},
	}

	statFunc = fakestat
	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			e = c.statResults
			eIndex = 0
			conf := findConfigFile(c.input)
			if conf != c.exp {
				t.Fatalf("Got %s expected %s", conf, c.exp)
			}
		})
	}
}

func TestGetConfigFiles(t *testing.T) {
	cases := []struct {
		config      map[string]interface{}
		exp         map[string]string
		statResults []error
	}{
		{
			config:      map[string]interface{}{"components": []string{"apiserver"}, "apiserver": map[string]interface{}{"confs": []string{"apiserver", "kube-apiserver"}}},
			statResults: []error{os.ErrNotExist, nil},
			exp:         map[string]string{"apiserver": "kube-apiserver"},
		},
		{
			// Component "thing" isn't included in the list of components
			config: map[string]interface{}{
				"components": []string{"apiserver"},
				"apiserver":  map[string]interface{}{"confs": []string{"apiserver", "kube-apiserver"}},
				"thing":      map[string]interface{}{"confs": []string{"/my/file/thing"}}},
			statResults: []error{os.ErrNotExist, nil},
			exp:         map[string]string{"apiserver": "kube-apiserver"},
		},
		{
			// More than one component
			config: map[string]interface{}{
				"components": []string{"apiserver", "thing"},
				"apiserver":  map[string]interface{}{"confs": []string{"apiserver", "kube-apiserver"}},
				"thing":      map[string]interface{}{"confs": []string{"/my/file/thing"}}},
			statResults: []error{os.ErrNotExist, nil, nil},
			exp:         map[string]string{"apiserver": "kube-apiserver", "thing": "/my/file/thing"},
		},
		{
			// Default thing to specified default config
			config: map[string]interface{}{
				"components": []string{"apiserver", "thing"},
				"apiserver":  map[string]interface{}{"confs": []string{"apiserver", "kube-apiserver"}},
				"thing":      map[string]interface{}{"confs": []string{"/my/file/thing"}, "defaultconf": "another/thing"}},
			statResults: []error{os.ErrNotExist, nil, os.ErrNotExist},
			exp:         map[string]string{"apiserver": "kube-apiserver", "thing": "another/thing"},
		},
		{
			// Default thing to component name
			config: map[string]interface{}{
				"components": []string{"apiserver", "thing"},
				"apiserver":  map[string]interface{}{"confs": []string{"apiserver", "kube-apiserver"}},
				"thing":      map[string]interface{}{"confs": []string{"/my/file/thing"}}},
			statResults: []error{os.ErrNotExist, nil, os.ErrNotExist},
			exp:         map[string]string{"apiserver": "kube-apiserver", "thing": "thing"},
		},
	}

	v := viper.New()
	statFunc = fakestat

	for id, c := range cases {
		t.Run(strconv.Itoa(id), func(t *testing.T) {
			for k, val := range c.config {
				v.Set(k, val)
			}
			e = c.statResults
			eIndex = 0

			m := getConfigFiles(v)
			if !reflect.DeepEqual(m, c.exp) {
				t.Fatalf("Got %v\nExpected %v", m, c.exp)
			}
		})
	}
}

func TestMakeSubsitutions(t *testing.T) {
	cases := []struct {
		input string
		subst map[string]string
		exp   string
	}{
		{input: "Replace $thisbin", subst: map[string]string{"this": "that"}, exp: "Replace that"},
		{input: "Replace $thisbin", subst: map[string]string{"this": "that", "here": "there"}, exp: "Replace that"},
		{input: "Replace $thisbin and $herebin", subst: map[string]string{"this": "that", "here": "there"}, exp: "Replace that and there"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			s := makeSubstitutions(c.input, "bin", c.subst)
			if s != c.exp {
				t.Fatalf("Got %s expected %s", s, c.exp)
			}
		})
	}
}