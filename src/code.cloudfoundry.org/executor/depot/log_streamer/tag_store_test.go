package log_streamer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("tagStore", func() {
	It("returns the stored tags", func() {
		store := newTagStore(map[string]string{"app_name": "some-app"})
		Expect(store.Load()).To(Equal(map[string]string{"app_name": "some-app"}))

		store.Store(map[string]string{"app_name": "other-app"})
		Expect(store.Load()).To(Equal(map[string]string{"app_name": "other-app"}))
	})

	Context("when nil is stored", func() {
		It("returns an empty map", func() {
			store := newTagStore(nil)
			Expect(store.Load()).NotTo(BeNil())
			Expect(store.Load()).To(BeEmpty())

			store = newTagStore(map[string]string{"app_name": "some-app"})
			store.Store(nil)
			Expect(store.Load()).NotTo(BeNil())
			Expect(store.Load()).To(BeEmpty())
		})
	})

	Context("when the caller modifies its map after storing it", func() {
		It("does not affect the stored tags", func() {
			tags := map[string]string{"app_name": "some-app"}
			store := newTagStore(tags)
			tags["app_name"] = "modified"
			tags["extra"] = "value"
			Expect(store.Load()).To(Equal(map[string]string{"app_name": "some-app"}))

			tags = map[string]string{"app_name": "other-app"}
			store.Store(tags)
			delete(tags, "app_name")
			Expect(store.Load()).To(Equal(map[string]string{"app_name": "other-app"}))
		})
	})

	Context("when new tags are stored", func() {
		It("does not modify a previously loaded snapshot", func() {
			store := newTagStore(map[string]string{"app_name": "some-app"})
			snapshot := store.Load()

			store.Store(map[string]string{"app_name": "other-app"})

			Expect(snapshot).To(Equal(map[string]string{"app_name": "some-app"}))
			Expect(store.Load()).To(Equal(map[string]string{"app_name": "other-app"}))
		})
	})
})
