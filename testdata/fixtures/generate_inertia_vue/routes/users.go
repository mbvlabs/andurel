package routes

import (
	"github.com/mbvlabs/andurel/pkg/routing"
)

const UserPrefix = "/users"

var SessionNew = routing.NewSimpleRoute(
	"/sign-in",
	"users.new_user_session",
	UserPrefix,
	routing.InertiaRoute(),
)

var SessionCreate = routing.NewSimpleRoute(
	"/sign-in",
	"users.user_session",
	UserPrefix,
	routing.InertiaRoute(),
)

var SessionDestroy = routing.NewSimpleRoute(
	"/sign-out",
	"users.destroy_user_session",
	UserPrefix,
	routing.InertiaRoute(),
)

var PasswordNew = routing.NewSimpleRoute(
	"/password/new",
	"users.new_user_password",
	UserPrefix,
	routing.InertiaRoute(),
)

var PasswordCreate = routing.NewSimpleRoute(
	"/password",
	"users.user_password",
	UserPrefix,
	routing.InertiaRoute(),
)

var PasswordEdit = routing.NewRouteWithToken(
	"/password/:token/edit",
	"users.edit_user_password",
	UserPrefix,
	routing.InertiaRoute(),
)

var PasswordUpdate = routing.NewSimpleRoute(
	"/password",
	"users.user_password",
	UserPrefix,
	routing.InertiaRoute(),
)

var RegistrationNew = routing.NewSimpleRoute(
	"/sign-up",
	"users.new_user_registration",
	UserPrefix,
	routing.InertiaRoute(),
)

var RegistrationCreate = routing.NewSimpleRoute(
	"",
	"users.user_registration",
	UserPrefix,
	routing.InertiaRoute(),
)

var ConfirmationNew = routing.NewSimpleRoute(
	"/confirmation/new",
	"users.new_user_confirmation",
	UserPrefix,
	routing.InertiaRoute(),
)

var ConfirmationCreate = routing.NewSimpleRoute(
	"/confirmation",
	"users.user_confirmation",
	UserPrefix,
	routing.InertiaRoute(),
)
