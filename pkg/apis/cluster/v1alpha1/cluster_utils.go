package v1alpha1

// String returns a well-formatted string for the Cluster object.
func (c *Cluster) String() string {
	return c.Name
}
