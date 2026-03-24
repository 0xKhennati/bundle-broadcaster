package relays

import "github.com/0xKhennati/bundle-broadcaster/strategies"

func init() {
	strategies.RegisterRelay("flashbots", &FlashbotsBuilder{})
	strategies.RegisterRelay("titanbuilder", &TitanbuilderBuilder{})
	strategies.RegisterRelay("quasar", &QuasarBuilder{})
	strategies.RegisterRelay("bobthebuilder", &BobthebuilderBuilder{})
	strategies.RegisterRelay("beaverbuild", &BeaverbuildBuilder{})
	strategies.RegisterRelay("buildernet", &BuildernetBuilder{})
	strategies.RegisterRelay("eurekabuilder", &EurekabuilderBuilder{})
	strategies.RegisterRelay("rsyncbuilder", &RsyncbuilderBuilder{})
	strategies.RegisterRelay("jetbldr", &JetbldrBuilder{})
	strategies.RegisterRelay("tbuilder", &TbuilderBuilder{})
	strategies.RegisterRelay("turbobuilder", &TurbobuilderBuilder{})
	strategies.RegisterRelay("snailbuilder", &SnailbuilderBuilder{})
}
