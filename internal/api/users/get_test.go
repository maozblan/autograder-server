package users

import (
	"reflect"
	"testing"

	"github.com/edulinq/autograder/internal/api/core"
	"github.com/edulinq/autograder/internal/db"
	"github.com/edulinq/autograder/internal/model"
	"github.com/edulinq/autograder/internal/timestamp"
	"github.com/edulinq/autograder/internal/util"
)

func TestGetBase(test *testing.T) {
	users := db.MustGetServerUsers()

	testCases := []struct {
		contextUser *model.ServerUser
		target      string
		permError   bool
		expected    *model.ServerUser
	}{
		// Self, empty.
		{users["course-student@test.edulinq.org"], "", false, users["course-student@test.edulinq.org"]},
		{users["server-creator@test.edulinq.org"], "", false, users["server-creator@test.edulinq.org"]},
		{users["server-admin@test.edulinq.org"], "", false, users["server-admin@test.edulinq.org"]},

		// Other, bad permissions.
		{users["server-creator@test.edulinq.org"], "course-student@test.edulinq.org", true, nil},

		// Other, good permissions.
		{users["server-admin@test.edulinq.org"], "course-student@test.edulinq.org", false, users["course-student@test.edulinq.org"]},

		// Missing
		{users["server-creator@test.edulinq.org"], "ZZZ", true, nil},
		{users["server-admin@test.edulinq.org"], "ZZZ", false, nil},
	}

	for i, testCase := range testCases {
		fields := map[string]any{
			"user-email":   testCase.contextUser.Email,
			"user-pass":    util.Sha256HexFromString(*testCase.contextUser.Name),
			"target-email": testCase.target,
		}

		response := core.SendTestAPIRequest(test, `users/get`, fields)
		if !response.Success {
			if testCase.permError {
				expectedLocator := "-046"
				if response.Locator != expectedLocator {
					test.Errorf("Case %d: Incorrect error returned. Expected '%s', found '%s'.",
						i, expectedLocator, response.Locator)
				}
			} else {
				test.Errorf("Case %d: Response is not a success when it should be: '%v'.", i, response)
			}

			continue
		}

		if testCase.permError {
			test.Errorf("Case %d: Did not get an expected permissions error.", i)
			continue
		}

		var responseContent GetResponse
		util.MustJSONFromString(util.MustToJSON(response.Content), &responseContent)

		expectedFound := (testCase.expected != nil)
		if expectedFound != responseContent.Found {
			test.Errorf("Case %d: Found user does not match. Expected: '%v', actual: '%v'.", i, expectedFound, responseContent.Found)
			continue
		}

		if testCase.expected == nil {
			continue
		}

		expectedInfo := core.NewServerUserInfo(testCase.expected)
		if !reflect.DeepEqual(expectedInfo, responseContent.User) {
			test.Errorf("Case %d: Unexpected user result. Expected: '%s', actual: '%s'.",
				i, util.MustToJSONIndent(expectedInfo), util.MustToJSONIndent(responseContent.User))
			continue
		}
	}
}

func TestGetSingle(test *testing.T) {
	response := core.SendTestAPIRequestFull(test, `users/get`, nil, nil, "course-admin")
	if !response.Success {
		test.Fatalf("Response is not a success when it should be: '%v'.", response)
	}

	var responseContent GetResponse
	util.MustJSONFromString(util.MustToJSON(response.Content), &responseContent)

	expected := GetResponse{
		Found: true,
		User: &core.ServerUserInfo{
			BaseUserInfo: core.BaseUserInfo{
				Type:  core.ServerUserInfoType,
				Email: "course-admin@test.edulinq.org",
				Name:  "course-admin",
			},
			Role: model.ServerRoleUser,
			Courses: map[string]core.EnrollmentInfo{
				"course-languages": core.EnrollmentInfo{
					CourseID: "course-languages",
					Role:     model.CourseRoleAdmin,
				},
				"course101": core.EnrollmentInfo{
					CourseID: "course101",
					Role:     model.CourseRoleAdmin,
				},
			},
		},
		Courses: map[string]*core.CourseInfo{
			"course-languages": &core.CourseInfo{
				ID:   "course-languages",
				Name: "Course Using Different Languages",
				Assignments: map[string]*core.AssignmentInfo{
					"bash": &core.AssignmentInfo{
						ID:      "bash",
						Name:    "A Simple Bash Assignment",
						DueDate: timestamp.ZeroPointer(),
					},
					"cpp-simple": &core.AssignmentInfo{
						ID:   "cpp-simple",
						Name: "A Simple C++ Assignment",
					},
					"java": &core.AssignmentInfo{
						ID:   "java",
						Name: "A Simple Java Assignment",
					},
				},
			},
			"course101": &core.CourseInfo{
				ID:   "course101",
				Name: "Course 101",
				Assignments: map[string]*core.AssignmentInfo{
					"hw0": &core.AssignmentInfo{
						ID:   "hw0",
						Name: "Homework 0",
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(expected, responseContent) {
		test.Fatalf("Unexpected result. Expected: '%s', actual: '%s'.",
			util.MustToJSONIndent(expected), util.MustToJSONIndent(responseContent))
	}
}
