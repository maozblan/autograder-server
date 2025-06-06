package canvas

import (
	"fmt"

	"github.com/edulinq/autograder/internal/lms/lmstypes"
	"github.com/edulinq/autograder/internal/util"
)

func (this *CanvasBackend) FetchAssignment(assignmentID string) (*lmstypes.Assignment, error) {
	if assignmentID == "" {
		return nil, fmt.Errorf("Cannot fetch assignment, target assignment ID is empty.")
	}

	this.getAPILock()
	defer this.releaseAPILock()

	apiEndpoint := fmt.Sprintf(
		"/api/v1/courses/%s/assignments/%s",
		this.CourseID, assignmentID)
	url := this.BaseURL + apiEndpoint

	headers := this.standardHeaders()
	body, _, err := util.GetWithHeaders(url, headers)

	if err != nil {
		return nil, fmt.Errorf("Failed to fetch assignment: '%w'.", err)
	}

	var assignment Assignment
	err = util.JSONFromString(body, &assignment)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal assignment: '%w'.", err)
	}

	return assignment.ToLMSType(), nil
}

func (this *CanvasBackend) FetchAssignments() ([]*lmstypes.Assignment, error) {
	return this.fetchAssignments(false)
}

func (this *CanvasBackend) fetchAssignments(rewriteLinks bool) ([]*lmstypes.Assignment, error) {
	this.getAPILock()
	defer this.releaseAPILock()

	apiEndpoint := fmt.Sprintf(
		"/api/v1/courses/%s/assignments?per_page=%d",
		this.CourseID, PAGE_SIZE)
	url := this.BaseURL + apiEndpoint

	headers := this.standardHeaders()

	assignments := make([]*lmstypes.Assignment, 0)

	for url != "" {
		var err error

		if rewriteLinks {
			url, err = this.rewriteLink(url)
			if err != nil {
				return nil, err
			}
		}

		body, responseHeaders, err := util.GetWithHeaders(url, headers)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch users: '%w'.", err)
		}

		var pageAssignments []*Assignment
		err = util.JSONFromString(body, &pageAssignments)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal assignments page: '%w'.", err)
		}

		for _, assignment := range pageAssignments {
			if assignment == nil {
				continue
			}

			assignments = append(assignments, assignment.ToLMSType())
		}

		url = fetchNextCanvasLink(responseHeaders)
	}

	return assignments, nil
}
