package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman healthcheck run", func() {

	It("podman healthcheck run bogus container", func() {
		session := podmanTest.Podman([]string{"healthcheck", "run", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman disable healthcheck with --no-healthcheck on valid container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--no-healthcheck", "--name", "hc", HEALTHCHECK_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman disable healthcheck with --no-healthcheck must not show starting on status", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--no-healthcheck", "--name", "hc", HEALTHCHECK_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Health}}", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())
		Expect(hc.OutputToString()).To(Equal("<nil>"))
	})

	It("podman run healthcheck and logs should contain healthcheck output", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test-logs", "-dt", "--health-interval", "1s", "--health-cmd", "echo working", "busybox", "sleep", "3600"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Buy a little time to get container running
		for i := 0; i < 5; i++ {
			hc := podmanTest.Podman([]string{"healthcheck", "run", "test-logs"})
			hc.WaitWithDefaultTimeout()
			exitCode := hc.ExitCode()
			if exitCode == 0 || i == 4 {
				break
			}
			time.Sleep(1 * time.Second)
		}

		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Healthcheck.Log}}", "test-logs"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())
		Expect(hc.OutputToString()).To(ContainSubstring("working"))
	})

	It("podman healthcheck from image's config (not container config)", func() {
		// Regression test for #12226: a health check may be defined in
		// the container or the container-config of an image.
		session := podmanTest.Podman([]string{"create", "-q", "--name", "hc", "quay.io/libpod/healthcheck:config-only", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hc := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Healthcheck}}", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())
		Expect(hc.OutputToString()).To(Equal("{[CMD-SHELL curl -f http://localhost/ || exit 1] 0s 5m0s 3s 0}"))
	})

	It("podman disable healthcheck with --health-cmd=none on valid container", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "none", "--name", "hc", HEALTHCHECK_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck on valid container", func() {
		Skip("Extremely consistent flake - re-enable on debugging")
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", HEALTHCHECK_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		exitCode := 999

		// Buy a little time to get container running
		for i := 0; i < 5; i++ {
			hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
			hc.WaitWithDefaultTimeout()
			exitCode = hc.ExitCode()
			if exitCode == 0 || i == 4 {
				break
			}
			time.Sleep(1 * time.Second)
		}
		Expect(exitCode).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring("(healthy)"))
	})

	It("podman healthcheck that should fail", func() {
		session := podmanTest.Podman([]string{"run", "-q", "-dt", "--name", "hc", "quay.io/libpod/badhealthcheck:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))
	})

	It("podman healthcheck on stopped container", func() {
		session := podmanTest.Podman([]string{"run", "--name", "hc", HEALTHCHECK_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck on container without healthcheck", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(125))
	})

	It("podman healthcheck should be starting", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))
	})

	It("podman healthcheck failed checks in start-period should not change status", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))
		// test old podman compat (see #11645)
		Expect(inspect[0].State.Healthcheck()).To(HaveField("Status", "starting"))
	})

	It("podman healthcheck failed checks must reach retries before unhealthy ", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckUnhealthy))
		// test old podman compat (see #11645)
		Expect(inspect[0].State.Healthcheck()).To(HaveField("Status", define.HealthCheckUnhealthy))
	})

	It("podman healthcheck good check results in healthy even in start-period", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-start-period", "2m", "--health-retries", "2", "--health-cmd", "ls || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))
	})

	It("podman healthcheck unhealthy but valid arguments check", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "[\"ls\", \"/foo\"]", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))
	})

	It("podman healthcheck single healthy result changes failed to healthy", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-retries", "2", "--health-cmd", "ls /foo || exit 1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect := podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckUnhealthy))

		foo := podmanTest.Podman([]string{"exec", "hc", "touch", "/foo"})
		foo.WaitWithDefaultTimeout()
		Expect(foo).Should(ExitCleanly())

		hc = podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())

		inspect = podmanTest.InspectContainer("hc")
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))

		// Test that events generated have correct status (#19237)
		events := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=health_status", "--since", "1m"})
		events.WaitWithDefaultTimeout()
		Expect(events).Should(ExitCleanly())
		eventsOut := events.OutputToStringArray()
		Expect(eventsOut).To(HaveLen(3))
		Expect(eventsOut[0]).To(ContainSubstring("health_status=starting"))
		Expect(eventsOut[1]).To(ContainSubstring("health_status=unhealthy"))
		Expect(eventsOut[2]).To(ContainSubstring("health_status=healthy"))

		// Test podman ps --filter health is working (#11687)
		ps := podmanTest.Podman([]string{"ps", "--filter", "health=healthy"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToStringArray()).To(HaveLen(2))
		Expect(ps.OutputToString()).To(ContainSubstring("hc"))
	})

	It("hc logs do not include exec events", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-cmd", "true", "--health-interval", "5s", "alpine", "sleep", "60"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hc := podmanTest.Podman([]string{"healthcheck", "run", "hc"})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())
		hcLogs := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "container=hc", "--filter", "event=exec_died", "--filter", "event=exec", "--since", "1m"})
		hcLogs.WaitWithDefaultTimeout()
		Expect(hcLogs).Should(ExitCleanly())
		hcLogsArray := hcLogs.OutputToStringArray()
		Expect(hcLogsArray).To(BeEmpty())
	})

	It("stopping and then starting a container with healthcheck cmd", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--name", "hc", "--health-cmd", "[\"ls\", \"/foo\"]", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stop := podmanTest.Podman([]string{"stop", "-t0", "hc"})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(ExitCleanly())

		startAgain := podmanTest.Podman([]string{"start", "hc"})
		startAgain.WaitWithDefaultTimeout()
		Expect(startAgain).Should(ExitCleanly())
		Expect(startAgain.OutputToString()).To(Equal("hc"))
		Expect(startAgain.ErrorToString()).To(Equal(""))
	})

	It("Verify default time is used and no utf-8 escapes", func() {
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.AddImageToRWStore(ALPINE)
		// Write target and fake files
		containerfile := fmt.Sprintf(`FROM %s
HEALTHCHECK CMD ls -l / 2>&1`, ALPINE)
		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0644)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			Expect(os.Chdir(cwd)).To(Succeed())
		}()

		// make cwd as context root path
		Expect(os.Chdir(podmanTest.TempDir)).To(Succeed())

		session := podmanTest.Podman([]string{"build", "--format", "docker", "-t", "test", "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Check if image inspect contains CMD-SHELL generated by healthcheck.
		session = podmanTest.Podman([]string{"image", "inspect", "--format", "{{.Config.Healthcheck}}", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("CMD-SHELL"))

		run := podmanTest.Podman([]string{"run", "-dt", "--name", "hctest", "test", "ls"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		inspect := podmanTest.InspectContainer("hctest")
		// Check to make sure a default time value was added
		Expect(inspect[0].Config.Healthcheck.Timeout).To(BeNumerically("==", 30000000000))
		// Check to make sure a default time interval value was added
		Expect(inspect[0].Config.Healthcheck.Interval).To(BeNumerically("==", 30000000000))
		// Check to make sure characters were not coerced to utf8
		Expect(inspect[0].Config.Healthcheck).To(HaveField("Test", []string{"CMD-SHELL", "ls -l / 2>&1"}))
	})

	It("Startup healthcheck success transitions to regular healthcheck", func() {
		ctrName := "hcCtr"
		ctrRun := podmanTest.Podman([]string{"run", "-dt", "--name", ctrName, "--health-cmd", "echo regular", "--health-startup-cmd", "cat /test", ALPINE, "top"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())

		inspect := podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))

		hc := podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		exec := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "touch /test && echo startup > /test"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		hc = podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())

		inspect = podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))

		hc = podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())

		inspect = podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))

		// Test podman ps --filter health is working (#11687)
		ps := podmanTest.Podman([]string{"ps", "--filter", "health=healthy"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToStringArray()).To(HaveLen(2))
		Expect(ps.OutputToString()).To(ContainSubstring("hc"))
	})
})
