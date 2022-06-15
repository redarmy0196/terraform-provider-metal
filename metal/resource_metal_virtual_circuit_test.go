package metal

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/packethost/packngo"
)

func init() {
	resource.AddTestSweepers("metal_virtual_circuit", &resource.Sweeper{
		Name:         "metal_virtual_circuit",
		Dependencies: []string{},
		F:            testSweepVirtualCircuits,
	})
}

func testSweepVirtualCircuits(region string) error {
	log.Printf("[DEBUG] Sweeping VirtualCircuits")
	config, err := sharedConfigForRegion(region)
	if err != nil {
		return fmt.Errorf("[INFO][SWEEPER_LOG] Error getting configuration for sweeping VirtualCircuits: %s", err)
	}
	metal := config.(*packngo.Client)
	orgList, _, err := metal.Organizations.List(nil)
	if err != nil {
		return fmt.Errorf("[INFO][SWEEPER_LOG] Error getting organization list for sweeping VirtualCircuits: %s", err)
	}
	vcs := map[string]*packngo.VirtualCircuit{}
	for _, org := range orgList {
		conns, _, err := metal.Connections.OrganizationList(org.ID, &packngo.GetOptions{Includes: []string{"ports"}})
		if err != nil {
			return fmt.Errorf("[INFO][SWEEPER_LOG] Error getting connections list for sweeping VirtualCircuits: %s", err)
		}
		for _, conn := range conns {
			for _, port := range conn.Ports {
				for _, vc := range port.VirtualCircuits {
					if strings.HasPrefix(vc.Name, "tfacc-vc") {
						vcs[vc.ID] = &vc
					}
				}
			}
		}
	}
	for _, vc := range vcs {
		log.Printf("[INFO][SWEEPER_LOG] Deleting VirtualCircuit: %s", vc.Name)
		_, err := metal.VirtualCircuits.Delete(vc.ID)
		if err != nil {
			return fmt.Errorf("[INFO][SWEEPER_LOG] Error deleting VirtualCircuit: %s", err)
		}
	}

	return nil
}

func testAccCheckMetalVirtualCircuitDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*packngo.Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "metal_virtual_circuit" {
			continue
		}
		if _, _, err := client.VirtualCircuits.Get(rs.Primary.ID, nil); err == nil {
			return fmt.Errorf("VirtualCircuit still exists")
		}
	}

	return nil
}

func testAccMetalVirtualCircuitConfig_Dedicated(randstr string, randint int) string {
	// Dedicated connection in DA metro
	testConnection := os.Getenv(metalDedicatedConnIDEnvVar)
	return fmt.Sprintf(`
data "metal_connection" "test" {
	connection_id = "%[1]s"
}

resource "metal_project" "test" {
	name = "%[4]s-pro-vc-%[2]s"
}

resource "metal_vlan" "test" {
	project_id  = metal_project.test.id
	metro       = "da"
	description = "%[4]s-vlan test"
}

resource "metal_virtual_circuit" "test" {
	name = "%[4]s-vc-%[2]s"
	description = "%[4]s-vc-%[2]s"
	connection_id = data.metal_connection.test.id
	project_id = metal_project.test.id
	port_id = data.metal_connection.test.ports[0].id
	vlan_id = metal_vlan.test.id
	nni_vlan = %[3]d
}
`, testConnection, randstr, randint, tstResourcePrefix)
}

func TestAccMetalVirtualCircuit_Dedicated(t *testing.T) {
	rs := acctest.RandString(10)
	ri := acctest.RandIntRange(1024, 1093)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckMetalVirtualCircuitDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMetalVirtualCircuitConfig_Dedicated(rs, ri),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"metal_virtual_circuit.test", "vlan_id",
						"metal_vlan.test", "id",
					),
				),
			},
			{
				ResourceName:            "metal_virtual_circuit.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"connection_id"},
			},
		},
	})
}
