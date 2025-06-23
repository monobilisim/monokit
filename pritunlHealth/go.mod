module github.com/monobilisim/monokit/pritunlHealth

go 1.24.0

toolchain go1.24.3

require (
	github.com/charmbracelet/lipgloss v1.0.0
	github.com/hashicorp/go-plugin v1.6.1
	github.com/monobilisim/monokit v0.0.0-00010101000000-000000000000
	github.com/muesli/termenv v0.15.2
	github.com/spf13/viper v1.19.0
	go.mongodb.org/mongo-driver/v2 v2.2.2
	google.golang.org/grpc v1.67.1
)

replace github.com/monobilisim/monokit => ../
replace github.com/hashicorp/go-plugin => github.com/hashicorp/go-plugin v1.6.0

