package server

// Filter guides for the rollout kit (ISSUE-131). Step 2 of the kit says
// "load the deny list into the filter you already run and point its block
// page at Sluss". These guides say exactly how, per product, with this
// instance's own URLs filled in, and say plainly where a product cannot
// redirect (Defender shows its own block notice) so the web clip and the
// managed bookmark carry the "offer" half instead.
//
// Click paths change between vendor releases; each guide names the screen,
// not pixel positions, and ends with the same check: open chatgpt.com on a
// managed device, then run the coverage script.

import (
	"fmt"
	"html/template"
	"strings"
)

// filterGuide is one product's walk-through.
type filterGuide struct {
	ID       string
	Name     string
	Fits     string // who typically runs it
	Redirect bool   // can the block send the user to /blocked
	Steps    []template.HTML
	Note     template.HTML
}

// guidesFor builds the guides with this instance's base URL in them.
func guidesFor(base string) []filterGuide {
	blocked, host := base+"/blocked", hostOf(base)
	file := func(name string) template.HTML {
		return template.HTML(`<a href="/router/rollout/file/` + name + `" download><code>` + name + `</code></a>`)
	}
	code := func(s string) template.HTML {
		return template.HTML("<code>" + template.HTMLEscapeString(s) + "</code>")
	}
	h := func(format string, a ...any) template.HTML { return template.HTML(fmt.Sprintf(format, a...)) }

	return []filterGuide{
		{
			ID: "defender", Name: "Microsoft Defender (Intune customers)", Fits: "Microsoft 365 E5 / Defender for Endpoint P2 with Intune",
			Redirect: false,
			Steps: []template.HTML{
				h(`<b>Defender for Cloud Apps, the managed way:</b> Cloud Apps → Cloud app catalog → filter category <i>Generative AI</i> → mark the apps you do not allow as <b>Unsanctioned</b>. With <i>Settings → Cloud Apps → Microsoft Defender for Endpoint → Enforce app access</i> on, every Intune-onboarded device blocks them through Network Protection, and new apps in the category can be unsanctioned the same way.`),
				h(`<b>Or exactly this list:</b> Settings → Endpoints → Indicators → <i>URLs/Domains</i> → Import, and upload %s — the deny list in Defender's import format, action <i>Block</i>.`, file("genai-defender-indicators.csv")),
				h(`Make sure Network Protection is in <b>block</b> mode (Intune → Endpoint security → Attack surface reduction, or the Defender antivirus policy), or Edge-only blocking applies to Chrome and Firefox too.`),
				h(`Leave %s alone — it is the company path.`, code(host)),
			},
			Note: h(`Defender shows its own block notice and cannot send the user to a custom page. Deploy the web clip and the browser policy from this step so the company chat is one click away, and say in the notice's custom text where to go.`),
		},
		{
			ID: "cloudflare", Name: "Cloudflare Zero Trust (Gateway)", Fits: "Cloudflare WARP on managed devices, or Gateway DNS",
			Redirect: true,
			Steps: []template.HTML{
				h(`My Team → Lists → Create list from CSV, type <i>Domains</i>, upload %s.`, file("genai-domains.csv")),
				h(`Gateway → Firewall policies → DNS (or HTTP when WARP inspects) → Add policy: selector <i>Domain</i> in list, action <b>Block</b>.`),
				h(`Settings → Custom pages → Block page → <b>Redirect to a URL</b>: %s. The policy's "Display block page" must be on.`, code(blocked+"?from=")),
				h(`Settings → WARP client → Device settings: the browser policy from this step turns DNS-over-HTTPS off so the browser cannot bypass Gateway.`),
			},
		},
		{
			ID: "umbrella", Name: "Cisco Umbrella", Fits: "Umbrella roaming client or network DNS",
			Redirect: true,
			Steps: []template.HTML{
				h(`Policies → Policy Components → Destination Lists → Add, type <i>Block</i>, and add the domains from %s (bulk upload takes the CSV's first column).`, file("genai-domains.csv")),
				h(`Policies → DNS Policies → your policy → Destination Lists: attach the list.`),
				h(`Policies → Policy Components → Block Page Appearance → Add: choose <b>Redirect to URL</b> and enter %s. Select that appearance in the DNS policy.`, code(blocked)),
				h(`Also in the DNS policy: Application Settings → <i>Generative AI</i> → block, to catch apps that are not on the list yet.`),
			},
		},
		{
			ID: "zscaler", Name: "Zscaler Internet Access", Fits: "Zscaler Client Connector",
			Redirect: true,
			Steps: []template.HTML{
				h(`Administration → URL Categories → Add a custom category and paste the domains from %s.`, file("genai-domains.csv")),
				h(`Policy → URL & Cloud App Control → Add rule: that category plus the cloud-app category <i>AI &amp; ML Applications</i>, action <b>Block</b>.`),
				h(`Administration → End User Notifications → <b>Custom notification</b> with redirect URL %s, so the block lands on the company AI page.`, code(blocked)),
				h(`Exempt %s from SSL inspection if your policy inspects by default — the chat streams answers and is already TLS end to end.`, code(host)),
			},
		},
		{
			ID: "dns", Name: "Any other DNS filter (NextDNS, Pi-hole, Infoblox, Windows DNS)", Fits: "a resolver you control",
			Redirect: false,
			Steps: []template.HTML{
				h(`Block or sinkhole the domains in %s. For Windows DNS, a zone per domain pointing at a sinkhole address works.`, file("genai-domains.csv")),
				h(`A sinkhole cannot show an HTTPS page for chatgpt.com without a certificate for it — the browser shows a connection error. That is expected, and is why the web clip and managed bookmark below carry the "offer".`),
				h(`Turn DNS-over-HTTPS off with the browser policy, or the browser resolves around your DNS.`),
			},
			Note: h(`If the filter can show a page over HTTPS (it has its own certificate on your managed devices), point it at %s.`, code(blocked)),
		},
	}
}

// defenderIndicators is the deny list in Defender for Endpoint's indicator
// import format (Settings → Endpoints → Indicators → Import).
func defenderIndicators(base string) string {
	var b strings.Builder
	b.WriteString("IndicatorType,IndicatorValue,ExpirationTime,Action,Severity,Title,Description,RecommendedActions,RbacGroups,Category,MitreTechniques,GenerateAlert\n")
	for _, d := range genAIDomains {
		fmt.Fprintf(&b, "DomainName,%s,,Block,Informational,Generative AI outside the company,Use the company AI chat at %s/chat,Open %s/chat,,,,FALSE\n",
			d.Domain, base, base)
	}
	return b.String()
}
