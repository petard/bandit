// Copyright 2013 SoundCloud, Rany Keddo. All rights reserved.  Use of this
// source code is governed by a license that can be found in the LICENSE file.

// Package http provides out of band handling for bandit tests. This can be
// used by a client side javascript app to determine arm selection and to
// record rewards.
package http

import (
	"encoding/json"
	"github.com/purzelrakete/bandit"
	"net/http"
	"strconv"
)

// APIResponse is the json response on the /test endpoint
type APIResponse struct {
	UID        string `json:"uid"`
	Experiment string `json:"experiment"`
	URL        string `json:"url"`
	Tag        string `json:"tag"`
}

// SelectionHandler can be used as an out of the box API endpoint for
// javascript applications.
//
// In this scenario, the application makes a request to the api endpoint:
//
//     GET https://api/test/widgets?uid=11 HTTP/1.0
//
// And receives a json response response
//
//     HTTP/1.0 200 OK
//     Content-Type: text/json
//
//     {
//       uid: 11,
//       experiment: "widgets",
//       url: "https://api/widget?color=blue"
//       tag: "widget-sauce-flf89"
//     }
//
// The client can now follow up with a request to the returned widget:
//
//     GET https://api/widget?color=blue HTTP/1.0
//
// This two phase approach can be collapsed by using the bandit directly
// inside a golang api endpoint.
func SelectionHandler(tests bandit.Tests) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "text/json")

		name := r.URL.Query().Get(":experiment")
		test, ok := tests[name]
		if ok != true {
			http.Error(w, "invalid experiment", http.StatusBadRequest)
			return
		}

		selected := test.Bandit.SelectArm()
		variant, err := bandit.SelectVariant(test.Experiment, selected)
		if err != nil {
			http.Error(w, "could not select variant", http.StatusInternalServerError)
			return
		}

		json, err := json.Marshal(APIResponse{
			UID:        "0",
			Experiment: test.Experiment.Name,
			URL:        variant.URL,
			Tag:        variant.Tag,
		})

		if err != nil {
			http.Error(w, "could not build variant", http.StatusInternalServerError)
			return
		}

		bandit.LogSelection("0", test.Experiment, variant)
		w.Write(json)
	}
}

// LogRewardHandler logs reward lines. It's better to log rewards directly
// through your main logging pipeline, but the handler is here in case you
// can't do that. This handler is currently updates the supplied bandits
// directly, which makes it unsuitable for real use.
func LogRewardHandler(tests bandit.Tests) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "text/application")

		tag := r.URL.Query().Get("tag")
		if tag == "" {
			http.Error(w, "cannot reward without tag", http.StatusBadRequest)
			return
		}

		reward := r.URL.Query().Get("reward")
		if reward == "" {
			http.Error(w, "reward missing", http.StatusBadRequest)
			return
		}

		fReward, err := strconv.ParseFloat(reward, 64)
		if err != nil {
			http.Error(w, "reward is not a float", http.StatusBadRequest)
			return
		}

		experiment, variant, err := bandit.GetVariant(&tests, tag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update the bandit in memory. This mechanism is not practical and will
		// be converted into a batch update scheme soon.
		b := tests[experiment.Name].Bandit
		b.Update(variant.Ordinal, fReward)

		bandit.LogReward("0", experiment, variant, fReward)
		w.WriteHeader(http.StatusOK)
	}
}
