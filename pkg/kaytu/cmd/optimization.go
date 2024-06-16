package cmd

import "time"

type Optimization struct {
	Command    string    `json:"command"`
	LastUpdate time.Time `json:"lastUpdate"`
}

type OptimizationsConfig struct {
	Optimizations []Optimization `json:"optimizations"`
}
