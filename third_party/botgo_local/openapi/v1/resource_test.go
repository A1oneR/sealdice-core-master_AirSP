package v1

import "testing"

func TestGetURL(t *testing.T) {
	tests := []struct {
		name    string
		sandbox bool
		want    string
	}{
		{name: "production", want: "https://api.bot.qq.com/gateway"},
		{name: "sandbox", sandbox: true, want: "https://sandbox.api.sgroup.qq.com/gateway"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &openAPI{sandbox: test.sandbox}
			if got := api.getURL(gatewayURI); got != test.want {
				t.Fatalf("getURL() = %q, want %q", got, test.want)
			}
		})
	}
}
