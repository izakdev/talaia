package availability

import (
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
	st "github.com/izakdev/talaia/internal/integrations/statetest"
)

func TestState(t *testing.T) {
	up := integrations.State{Tone: integrations.ToneGood, Key: "availability.state.up"}
	st.Run(t, []st.Case{
		st.C("tcp", Data{Check: "tcp", LatencyMS: 2}, up),
		st.C("http 200", Data{Check: "http", LatencyMS: 5, HTTPStatus: 200}, up),
		st.C("http 302", Data{Check: "http", LatencyMS: 5, HTTPStatus: 302}, up),
		st.C("http 503", Data{Check: "http", LatencyMS: 5, HTTPStatus: 503},
			integrations.State{Tone: integrations.ToneWarn, Key: "availability.state.httpError", Params: map[string]any{"code": 503}}),
		st.C("http 401", Data{Check: "http", LatencyMS: 5, HTTPStatus: 401},
			integrations.State{Tone: integrations.ToneWarn, Key: "availability.state.httpError", Params: map[string]any{"code": 401}}),
	})
}
