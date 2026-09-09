package ui_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/ui"
)

var _ = Describe("Camera", func() {
	Describe("coordinate transforms", func() {
		It("round-trips an arbitrary galaxy point via GalaxyToScreen/ScreenToGalaxy", func() {
			cam := ui.NewCamera(1920, 1080)
			gx, gy := float32(300), float32(700)
			sx, sy := cam.GalaxyToScreen(gx, gy)
			rx, ry := cam.ScreenToGalaxy(sx, sy)
			Expect(rx).To(BeNumerically("~", gx, 0.001))
			Expect(ry).To(BeNumerically("~", gy, 0.001))
		})

		It("maps the camera center to the screen center", func() {
			cam := ui.NewCamera(1920, 1080)
			sx, sy := cam.GalaxyToScreen(cam.CenterX, cam.CenterY)
			Expect(sx).To(BeNumerically("~", float32(960), 0.001))
			Expect(sy).To(BeNumerically("~", float32(540), 0.001))
		})

		It("screen distance doubles when zoom doubles", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = 2.0
			sx1, _ := cam.GalaxyToScreen(cam.CenterX, cam.CenterY)
			sx2, _ := cam.GalaxyToScreen(cam.CenterX+100, cam.CenterY)
			Expect(sx2 - sx1).To(BeNumerically("~", float32(200), 0.001))
		})
	})

	Describe("Tier", func() {
		It("returns ZoomFar below ZoomMidThreshold", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = 0.4
			Expect(cam.Tier()).To(Equal(ui.ZoomFar))
		})

		It("returns ZoomMid between the thresholds", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = 1.2
			Expect(cam.Tier()).To(Equal(ui.ZoomMid))
		})

		It("returns ZoomClose at or above ZoomCloseThreshold", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = 3.0
			Expect(cam.Tier()).To(Equal(ui.ZoomClose))
		})

		It("transitions from Far to Mid at ZoomMidThreshold", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = ui.ZoomMidThreshold - 0.001
			Expect(cam.Tier()).To(Equal(ui.ZoomFar))
			cam.Zoom = ui.ZoomMidThreshold
			Expect(cam.Tier()).To(Equal(ui.ZoomMid))
		})
	})

	Describe("ZoomIn / ZoomOut", func() {
		It("does not exceed ZoomMax after many ZoomIn calls", func() {
			cam := ui.NewCamera(1920, 1080)
			for range 200 {
				cam.ZoomIn()
			}
			Expect(cam.Zoom).To(BeNumerically("<=", ui.ZoomMax))
		})

		It("does not go below ZoomMin after many ZoomOut calls", func() {
			cam := ui.NewCamera(1920, 1080)
			for range 200 {
				cam.ZoomOut()
			}
			Expect(cam.Zoom).To(BeNumerically(">=", ui.ZoomMin))
		})
	})

	Describe("Pan", func() {
		It("moves CenterX by the requested delta", func() {
			cam := ui.NewCamera(1920, 1080)
			orig := cam.CenterX
			cam.Pan(50, 0)
			Expect(cam.CenterX).To(BeNumerically("~", orig+50, 0.001))
		})

		It("moves CenterY by the requested delta", func() {
			cam := ui.NewCamera(1920, 1080)
			orig := cam.CenterY
			cam.Pan(0, -30)
			Expect(cam.CenterY).To(BeNumerically("~", orig-30, 0.001))
		})
	})

	Describe("PanPx", func() {
		It("converts pixel delta to galaxy delta via zoom", func() {
			cam := ui.NewCamera(1920, 1080)
			cam.Zoom = 2.0
			orig := cam.CenterX
			cam.PanPx(100, 0)
			Expect(cam.CenterX).To(BeNumerically("~", orig+50, 0.001))
		})
	})

	Describe("Visible", func() {
		It("reports the camera center as visible", func() {
			cam := ui.NewCamera(1920, 1080)
			Expect(cam.Visible(cam.CenterX, cam.CenterY, 0)).To(BeTrue())
		})

		It("reports a far-off-screen point as not visible", func() {
			cam := ui.NewCamera(1920, 1080)
			Expect(cam.Visible(-9999, -9999, 0)).To(BeFalse())
		})
	})
})
