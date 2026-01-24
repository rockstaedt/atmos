package web

import "embed"

//go:embed templates/* templates/partials/* static/*
var Files embed.FS
