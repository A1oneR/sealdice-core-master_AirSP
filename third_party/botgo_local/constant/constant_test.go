package constant

import "testing"

func TestQQBotDomains(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "api", got: APIDomain, want: "https://api.bot.qq.com"},
		{name: "token", got: TokenDomain, want: "https://api.bot.qq.com"},
		{name: "sandbox", got: SandBoxAPIDomain, want: "https://sandbox.api.sgroup.qq.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("domain = %q, want %q", test.got, test.want)
			}
		})
	}
}
