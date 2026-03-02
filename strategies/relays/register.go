package relays

import "github.com/bundle-broadcaster/strategies"

func init() {
	strategies.RegisterRelay("flashbots", &FlashbotsBuilder{})
	strategies.RegisterRelay("titanbuilder", &TitanbuilderBuilder{})
	strategies.RegisterRelay("quasar", &QuasarBuilder{})
	strategies.RegisterRelay("bobthebuilder", &BobthebuilderBuilder{})
	strategies.RegisterRelay("beaverbuild", &BeaverbuildBuilder{})
	strategies.RegisterRelay("buildernet", &BuildernetBuilder{})
}
