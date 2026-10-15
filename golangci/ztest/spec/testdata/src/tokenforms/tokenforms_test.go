package tokenforms_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"tokenforms"
	"tokenforms/states"
)

func TestRendered(t *testing.T) {
	job := tokenforms.Job{State: states.StateRunning, Mode: states.ModeFast}

	Expect(job.State.String()).To(Equal("running"))                  // want `^token-assertion: asserts the rendered token of states.State, which only the spec of the file declaring it or its String pins; assert the member instead`
	Expect(fmt.Sprint(job.State)).To(Equal("running"))               // want `^token-assertion: asserts the rendered token of states.State`
	Expect(string(job.Mode)).To(Equal("fast"))                       // want `^token-assertion: asserts the rendered token of states.Mode`
	Expect(strings.ToUpper(job.State.String())).To(Equal("RUNNING")) // want `^token-assertion: asserts the rendered token of states.State`
	Expect(job.State.String()[:3]).To(Equal("run"))                  // want `^token-assertion: asserts the rendered token of states.State`
	Expect(job).To(HaveField("State.String()", "running"))           // want `^token-assertion: asserts the rendered token of states.State`
	Expect(&job).To(HaveField("State.String()", Equal("running")))   // want `^token-assertion: asserts the rendered token of states.State`
	assert.Equal(t, "running", job.State.String())                   // want `^token-assertion: asserts the rendered token of states.State`

	name := job.State.String()
	Expect(name).To(ContainSubstring("run")) // want `^token-assertion: asserts the rendered token of states.State`
}

func TestIdentity(t *testing.T) {
	job := tokenforms.Job{State: states.StateRunning}

	Expect(job.State).To(Equal(states.StateRunning))
	Expect(job).To(HaveField("State", states.StateRunning))
	assert.Equal(t, states.StateRunning, job.State)
}

func TestNegated(t *testing.T) {
	job := tokenforms.Job{State: states.StateRunning}

	Expect(job.State.String()).NotTo(Equal("pending"))
	Expect(job).NotTo(HaveField("State.String()", "pending"))
	assert.NotEqual(t, "pending", job.State.String())
}

func TestNoEnum(t *testing.T) {
	var label states.Label

	Expect(states.FlagA.String()).To(Equal("flags"))
	Expect(states.MB.String()).To(Equal("size"))
	Expect(label.String()).To(Equal("label"))
	Expect(time.January.String()).To(Equal("January"))

	mixed := states.StateRunning.String()
	mixed = label.String()
	Expect(mixed).To(Equal("label"))
}
