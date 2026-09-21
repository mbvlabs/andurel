package controllers

import (
	"example.com/app/router"

	"go.uber.org/fx"
)

var constructors = fx.Provide()

var Module = fx.Module(
	"controllers",
	constructors,
)
