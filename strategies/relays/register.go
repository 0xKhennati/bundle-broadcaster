package relays

import "github.com/0xKhennati/bundle-broadcaster/strategies"

func init() {
	// Original builders
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

	// Builders from Flashbots DOWG list (https://github.com/flashbots/dowg)
	strategies.RegisterRelay("f1b", &F1bBuilder{})
	strategies.RegisterRelay("builder0x69", &Builder0x69Builder{})
	strategies.RegisterRelay("eigenphi", &EigenphiBuilder{})
	strategies.RegisterRelay("boba", &BobaBuilder{})
	strategies.RegisterRelay("gambit", &GambitBuilder{})
	strategies.RegisterRelay("payload", &PayloadBuilder{})
	strategies.RegisterRelay("loki", &LokiBuilder{})
	strategies.RegisterRelay("buildai", &BuildaiBuilder{})
	strategies.RegisterRelay("penguin", &PenguinBuilder{})
	strategies.RegisterRelay("btcs", &BtcsBuilder{})
	strategies.RegisterRelay("bloxroute", &BloxrouteBuilder{})
	strategies.RegisterRelay("blockbeelder", &BlockbeelderBuilder{})
}
