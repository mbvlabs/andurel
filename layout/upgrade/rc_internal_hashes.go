package upgrade

// knownRCInternalTemplateHashes was derived from the exact RC.1, RC.2, and RC.3
// scaffold goldens. Module paths are normalized before hashing.
var knownRCInternalTemplateHashes = map[string]map[string]struct{}{
	"internal/hypermedia/broadcaster.go": hashSet(
		"8af19d4cc51070b1674e45395fa950fcf798adfc671a4b28e9753f99db32d0d3",
	),
	"internal/hypermedia/core.go": hashSet(
		"d610bccd719d8652a6271031b36502419e1c74a2a26f77e00c2fd5fd531bba82",
	),
	"internal/hypermedia/helpers.go": hashSet(
		"df33b584f933a954f57ed6966b332dfb3ec50e97f1090893ceeb830b3cda4e01",
	),
	"internal/hypermedia/options.go": hashSet(
		"96467434559f6bfe6409faae399fbdf658b13272095b74803f3712540288e83d",
	),
	"internal/hypermedia/render.go": hashSet(
		"723ab6080acb6e147697009899bebd193975c7498d9818d3766c80147b8ef2cd",
	),
	"internal/hypermedia/script.go": hashSet(
		"1ad9c3e80d950302e0b58eee10fd2a94bdbba927f05e191b20f16dfb1d515af9",
	),
	"internal/hypermedia/signals.go": hashSet(
		"b9f326693bcc3a56401af1f5b86b0107fe059c3e53337f52e1f30c72ada4e212",
	),
	"internal/hypermedia/sse.go": hashSet(
		"9a640cacfb0ef1505affd648837c22c6620221c22d7937c466c423fa06700574",
	),
	"internal/inertia/page_options.go": hashSet(
		"89d0ad45ca7621276fd039700c24a6d03eb0d0b7e98e5427b5dd48836f484e73",
	),
	"internal/inertia/render.go": hashSet(
		"c179ced4c13829213fbf8059a14e8235b5339a3ac648c41523394dcca8d18a6f",
		"c7acedda0f08157ecd0bb4b2780cc4c31583c765dc6becd7f6df1ffff97194a7",
	),
	"internal/inertia/vite.go": hashSet(
		"52c1ed35d1ae17fa16d2eea7b393c96aec55f0aea247b2f991ff1ce984a05917",
		"68db18a97e5f1889793a49c374af080e86d96cf06989eafeede8c5313f05b2ef",
	),
	"internal/request/context.go": hashSet(
		"3ebcdfb8c2429c0f4d38c63441c030730d2da3a4b02dbf57d3a0e901d20c4393",
	),
	"internal/request/request.go": hashSet(
		"8d04f14d7752d8e6f4af9acaca242b51048d05a698fe119c2a2bd34962cb0717",
	),
	"internal/routing/definitions.go": hashSet(
		"9b3717ca50aad2e90eebf85e564cd69c03607ae6e2fa738561680956fa19ea43",
	),
	"internal/routing/routes.go": hashSet(
		"d800ebc3cfe6b84eb2b8ce1347f97d3283cf4916606a362e638eccbe7597d62d",
	),
	"internal/server/server.go": hashSet(
		"12ad4d1af1dbb6db6fd7afeb44d4c130d36684d5e671063c6377ef29dee3d520",
	),
	"internal/storage/psql.go": hashSet(
		"ff6bdacbcadd70242ac42ad269e21461e8f195794189afbfd3d4f1d8ee88335a",
	),
	"internal/storage/queue.go": hashSet(
		"31c4214269da101c128292d07608d4d84da25ad9df262fee15886dd681e8639a",
	),
	"internal/validation/helpers.go": hashSet(
		"aff9eede9d7298e55ee3cfa7c2d58430c696341b6c850bab0be0d4f1e78f8b2a",
	),
	"internal/validation/rules.go": hashSet(
		"0b2a97f8e17032ba9995e8df47f1132aed00ac9c16666f982eac143638dc5394",
	),
	"internal/validation/validation.go": hashSet(
		"91a9deedca0f1aa5d0c3d2b326189e7e309417f57f1bd031ae666153f5c3c5e6",
	),
}
