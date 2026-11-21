package main

type target struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Seconds int    `json:"seconds"`
	Min     int    `json:"min"`
}

var targets = []target{
	{"flova", "Flova upload · parts up to 8 min", "upload", 478, 0},
	{"kling3", "Kling 3.0 Omni reference · up to 8 s", "ref", 8, 3},
	{"gemini", "Gemini Omni Flash reference · up to 10 s", "ref", 10, 0},
	{"wan3", "Wan 3.0 reference · up to 15 s", "ref", 15, 0},
	{"minimax", "MiniMax H3 reference · up to 15 s", "ref", 15, 0},
	{"seedance2", "Seedance 2.0 reference · up to 16 s", "ref", 16, 0},
	{"seedance25", "Seedance 2.5 reference · up to 30 s", "ref", 30, 0},
	{"kling4", "Kling 4.0 reference · up to 30 s", "ref", 30, 0},
}

func findTarget(id string) (target, bool) {
	for _, t := range targets {
		if t.ID == id {
			return t, true
		}
	}
	return target{}, false
}
