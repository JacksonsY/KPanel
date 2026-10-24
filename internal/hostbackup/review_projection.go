package hostbackup

import "github.com/kejilion/kejilion-panel/internal/redact"

// Security facts are derived from the exact execution configuration, then kept
// in this typed review schema. Project each untrusted text/configuration domain
// independently: a marker inside a container's text must not erase a different
// field's privileged flag, mount coverage, origin, or completeness decision.
// This function only handles review data; none of it is an execution input.
func sanitizeModuleReview(m *ModuleReview) {
	for i := range m.Roots {
		r := &m.Roots[i]
		r.Path = redact.Text(r.Path)
		for j := range r.UsedBy {
			r.UsedBy[j].Container = redact.Text(r.UsedBy[j].Container)
		}
	}
	for i := range m.Containers {
		c := &m.Containers[i]
		c.Name = redact.Text(c.Name)
		c.Image = redact.Text(c.Image)
		c.ImageID = redact.Text(c.ImageID)
		c.ImageRef = redact.Text(c.ImageRef)
		c.HostConfig = reviewJSON(c.HostConfig)
		c.CapAdd = redact.Records(c.CapAdd)
		c.Devices = redact.Records(c.Devices)
		c.HostNamespaces = redact.Records(c.HostNamespaces)
		c.SecurityOpt = redact.Records(c.SecurityOpt)
		for j := range c.Binds {
			c.Binds[j].Source = redact.Text(c.Binds[j].Source)
			c.Binds[j].Destination = redact.Text(c.Binds[j].Destination)
		}
	}
	for i := range m.Networks {
		n := &m.Networks[i]
		n.Name = redact.Text(n.Name)
		n.Driver = redact.Text(n.Driver)
		n.Subnets = redact.Records(n.Subnets)
		n.Options = redact.Records(n.Options)
		n.Config = reviewJSON(n.Config)
	}
	for i := range m.Volumes {
		v := &m.Volumes[i]
		v.Name = redact.Text(v.Name)
		v.Driver = redact.Text(v.Driver)
		v.Config = reviewJSON(v.Config)
	}
}
