// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0 OR ISC

package subprocess

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NIST ACVP EDDSA Schema: https://pages.nist.gov/ACVP/draft-celi-acvp-eddsa.html
type eddsa struct{}

func (e *eddsa) Process(vectorSet []byte, m Transactable) (interface{}, error) {
	var vs struct {
		Mode       string          `json:"mode"`
		TestGroups json.RawMessage `json:"testGroups"`
	}

	if err := json.Unmarshal(vectorSet, &vs); err != nil {
		return nil, err
	}

	var processTestGroups func(json.RawMessage, Transactable) (interface{}, error)

	switch {
	case strings.EqualFold(vs.Mode, "keyGen"):
		processTestGroups = processEddsaKeyGenTestGroup
	case strings.EqualFold(vs.Mode, "keyVer"):
		processTestGroups = processEddsaKeyVerTestGroup
	case strings.EqualFold(vs.Mode, "sigGen"):
		processTestGroups = processEddsaSigGenTestGroup
	case strings.EqualFold(vs.Mode, "sigVer"):
		processTestGroups = processEddsaSigVerTestGroup
	default:
		return nil, fmt.Errorf("unsupported EDDSA mode %q", vs.Mode)
	}

	return processTestGroups(vs.TestGroups, m)
}

func processEddsaKeyGenTestGroup(testGroups json.RawMessage, m Transactable) (interface{}, error) {
	var groups []eddsaKeyGenTestGroup
	if err := json.Unmarshal(testGroups, &groups); err != nil {
		return nil, err
	}

	var ret []eddsaKeyGenTestGroupResponse

	for _, group := range groups {
		response := eddsaKeyGenTestGroupResponse{
			ID: group.ID,
		}

		if group.Type != "AFT" {
			return nil, fmt.Errorf("unsupported test type %q", group.Type)
		}

		for _, test := range group.Tests {
			result, err := m.Transact("EDDSA/"+string(group.Curve)+"/keyGen", 2)
			if err != nil {
				return nil, err
			}

			response.Tests = append(response.Tests, eddsaKeyGenTestCaseResponse{
				ID: test.ID,
				D:  result[0],
				Q:  result[1],
			})
		}

		ret = append(ret, response)
	}

	return ret, nil
}

func processEddsaKeyVerTestGroup(testGroups json.RawMessage, m Transactable) (interface{}, error) {
	var groups []eddsaKeyVerTestGroup
	if err := json.Unmarshal(testGroups, &groups); err != nil {
		return nil, err
	}

	var ret []eddsaKeyVerTestGroupResponse

	for _, group := range groups {
		if group.Type != "AFT" {
			return nil, fmt.Errorf("unsupported test type %q", group.Type)
		}

		response := eddsaKeyVerTestGroupResponse{
			ID: group.ID,
		}

		for _, test := range group.Tests {
			results, err := m.Transact("EDDSA/"+string(group.Curve)+"/keyVer", 1, test.Q)
			if err != nil {
				return nil, err
			}

			var passed *bool
			if len(results[0]) == 1 {
				val := results[0][0] == 1
				passed = &val
			}

			response.Tests = append(response.Tests, eddsaKeyVerTestCaseResponse{
				ID:     test.ID,
				Passed: passed,
			})
		}

		ret = append(ret, response)
	}

	return ret, nil
}

func processEddsaSigGenTestGroup(testGroups json.RawMessage, m Transactable) (interface{}, error) {
	var groups []eddsaSigGenTestGroup
	if err := json.Unmarshal(testGroups, &groups); err != nil {
		return nil, err
	}

	var ret []eddsaSigGenTestGroupResponse

	for _, group := range groups {
		if group.Type != "AFT" && group.Type != "BFT" {
			return nil, fmt.Errorf("unsupported test type %q", group.Type)
		}

		results, err := m.Transact("EDDSA/"+string(group.Curve)+"/keyGen", 2)
		if err != nil {
			return nil, err
		}

		seed := results[0]

		response := eddsaSigGenTestGroupResponse{
			ID: group.ID,
			Q:  results[1],
		}

		for _, test := range group.Tests {
			results, err := m.Transact("EDDSA/"+string(group.Curve)+"/sigGen", 1, seed, test.Message)
			if err != nil {
				return nil, err
			}

			response.Tests = append(response.Tests, eddsaSigGenTestCaseResponse{
				ID:        test.ID,
				Signature: results[0],
			})
		}

		ret = append(ret, response)
	}

	return ret, nil
}

func processEddsaSigVerTestGroup(testGroups json.RawMessage, m Transactable) (interface{}, error) {
	var groups []eddsaSigVerTestGroup
	if err := json.Unmarshal(testGroups, &groups); err != nil {
		return nil, err
	}

	var ret []eddsaSigVerTestGroupResponse

	for _, group := range groups {
		if group.Type != "AFT" {
			return nil, fmt.Errorf("unsupported test type %q", group.Type)
		}

		response := eddsaSigVerTestGroupResponse{
			ID: group.ID,
		}

		for _, test := range group.Tests {
			results, err := m.Transact("EDDSA/"+string(group.Curve)+"/sigVer", 1, test.Message, test.Q, test.Signature)
			if err != nil {
				return nil, err
			}

			var passed *bool
			if len(results[0]) == 1 {
				val := results[0][0] == 1
				passed = &val
			}

			response.Tests = append(response.Tests, eddsaSigVerTestCaseResponse{
				ID:     test.ID,
				Passed: passed,
			})
		}

		ret = append(ret, response)
	}

	return ret, nil
}

const Ed25519 EDDSACurve = "ED-25519"

type EDDSACurve string

func (e *EDDSACurve) UnmarshalJSON(v []byte) error {
	var str string

	if err := json.Unmarshal(v, &str); err != nil {
		return err
	}

	switch {
	case strings.EqualFold(str, "ED-25519"):
		*e = Ed25519
	default:
		return fmt.Errorf("unsupported EDDSA curve: %q", str)
	}

	return nil
}

type eddsaKeyGenTestGroup struct {
	ID    uint64     `json:"tgId"`
	Curve EDDSACurve `json:"curve"`
	Type  string     `json:"testType"`
	Tests []struct {
		ID uint64 `json:"tcId"`
	}
}

type eddsaKeyVerTestGroup struct {
	ID    uint64     `json:"tgId"`
	Curve EDDSACurve `json:"curve"`
	Type  string     `json:"testType"`
	Tests []struct {
		ID uint64               `json:"tcId"`
		Q  hexEncodedByteString `json:"q"`
	}
}

type eddsaSigGenTestGroup struct {
	ID      uint64     `json:"tgId"`
	Curve   EDDSACurve `json:"curve"`
	Prehash bool       `json:"prehash"`
	Type    string     `json:"testType"`
	Tests   []struct {
		ID      uint64               `json:"tcId"`
		Message hexEncodedByteString `json:"message"`
	}
}

type eddsaSigVerTestGroup struct {
	ID      uint64     `json:"tgId"`
	Curve   EDDSACurve `json:"curve"`
	Prehash bool       `json:"prehash"`
	Type    string     `json:"testType"`
	Tests   []struct {
		ID        uint64               `json:"tcId"`
		Message   hexEncodedByteString `json:"message"`
		Q         hexEncodedByteString `json:"q"`
		Signature hexEncodedByteString `json:"signature"`
	}
}

type eddsaKeyGenTestGroupResponse struct {
	ID    uint64                        `json:"tgId"`
	Tests []eddsaKeyGenTestCaseResponse `json:"tests"`
}

type eddsaKeyGenTestCaseResponse struct {
	ID uint64               `json:"tcId"`
	D  hexEncodedByteString `json:"d"`
	Q  hexEncodedByteString `json:"q"`
}

type eddsaKeyVerTestGroupResponse struct {
	ID    uint64                        `json:"tgId"`
	Tests []eddsaKeyVerTestCaseResponse `json:"tests"`
}

type eddsaKeyVerTestCaseResponse struct {
	ID     uint64 `json:"tcId"`
	Passed *bool  `json:"testPassed"`
}

type eddsaSigGenTestGroupResponse struct {
	ID    uint64                        `json:"tgId"`
	Q     hexEncodedByteString          `json:"q"`
	Tests []eddsaSigGenTestCaseResponse `json:"tests"`
}

type eddsaSigGenTestCaseResponse struct {
	ID        uint64               `json:"tcId"`
	Signature hexEncodedByteString `json:"signature"`
}

type eddsaSigVerTestGroupResponse struct {
	ID    uint64                        `json:"tgId"`
	Tests []eddsaSigVerTestCaseResponse `json:"tests"`
}

type eddsaSigVerTestCaseResponse struct {
	ID     uint64 `json:"tcId"`
	Passed *bool  `json:"testPassed"`
}