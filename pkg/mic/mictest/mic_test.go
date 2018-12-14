package mictest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	aadpodid "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	cp "github.com/Azure/aad-pod-identity/pkg/cloudprovider/cloudprovidertest"
	"github.com/Azure/aad-pod-identity/pkg/config"
	crd "github.com/Azure/aad-pod-identity/pkg/crd/crdtest"
	pod "github.com/Azure/aad-pod-identity/pkg/pod/podtest"
	"github.com/golang/glog"
)

func testPositiveCases(t *testing.T, podNamespace string, idNamespace string, bindingNamespace string) {

	exit := make(<-chan struct{}, 0)
	eventCh := make(chan aadpodid.EventType, 100)
	cloudClient := cp.NewTestCloudClient(config.AzureConfig{})
	crdClient := crd.NewTestCrdClient(nil)
	podClient := pod.NewTestPodClient()
	nodeClient := NewTestNodeClient()
	var evtRecorder TestEventRecorder
	evtRecorder.lastEvent = new(LastEvent)

	micClient := NewMICClient(eventCh, cloudClient, crdClient, podClient, nodeClient, &evtRecorder)

	crdClient.CreateId("test-id", idNamespace, aadpodid.UserAssignedMSI, "test-user-msi-resourceid", "test-user-msi-clientid", nil, "", "", "")
	crdClient.CreateBinding("testbinding", bindingNamespace, "test-id", "test-select")

	nodeClient.AddNode("test-node")
	podClient.AddPod("test-pod", podNamespace, "test-node", "test-select")
	podClient.GetPods()

	eventCh <- aadpodid.PodCreated
	go micClient.Sync(exit)
	time.Sleep(2 * time.Second)
	testPass := false
	listAssignedIDs, err := crdClient.ListAssignedIDs()
	if err != nil {
		glog.Error(err)
		panic("list assigned failed")
	}
	if listAssignedIDs != nil {
		for _, assignedID := range *listAssignedIDs {
			if assignedID.Spec.Pod == "test-pod" && assignedID.Spec.PodNamespace == podNamespace && assignedID.Spec.NodeName == "test-node" &&
				assignedID.Spec.AzureBindingRef.Name == "testbinding" && assignedID.Spec.AzureIdentityRef.Name == "test-id" {
				testPass = evtRecorder.Validate(&LastEvent{Type: "Normal", Reason: "binding applied",
					Message: fmt.Sprintf("Binding testbinding applied on node test-node for pod test-pod-%s-test-id", podNamespace)})
				if !testPass {
					panic("event mismatch")
				}
				break
			}
		}
	}

	if !testPass {
		panic(fmt.Sprintf("assigned id mismatch for namespaces: pod: %s, binding: %s, id: %s", podNamespace, bindingNamespace, idNamespace))
	}
}

func testNegativeCases(t *testing.T, podNamespace string, idNamespace string, bindingNamespace string) {

	exit := make(<-chan struct{}, 0)
	eventCh := make(chan aadpodid.EventType, 100)
	cloudClient := cp.NewTestCloudClient(config.AzureConfig{})
	crdClient := crd.NewTestCrdClient(nil)
	podClient := pod.NewTestPodClient()
	nodeClient := NewTestNodeClient()
	var evtRecorder TestEventRecorder
	evtRecorder.lastEvent = new(LastEvent)

	micClient := NewMICClient(eventCh, cloudClient, crdClient, podClient, nodeClient, &evtRecorder)

	crdClient.CreateId("test-id", idNamespace, aadpodid.UserAssignedMSI, "test-user-msi-resourceid", "test-user-msi-clientid", nil, "", "", "")
	crdClient.CreateBinding("testbinding", bindingNamespace, "test-id", "test-select")

	nodeClient.AddNode("test-node")
	podClient.AddPod("test-pod", podNamespace, "test-node", "test-select")
	podClient.GetPods()

	eventCh <- aadpodid.PodCreated
	go micClient.Sync(exit)
	time.Sleep(2 * time.Second)
	testPass := false
	listAssignedIDs, err := crdClient.ListAssignedIDs()
	if err != nil {
		glog.Error(err)
		panic("list assigned failed")
	}
	if listAssignedIDs != nil {
		for _, assignedID := range *listAssignedIDs {
			if assignedID.Spec.Pod == "test-pod" && assignedID.Spec.PodNamespace == podNamespace && assignedID.Spec.NodeName == "test-node" &&
				assignedID.Spec.AzureBindingRef.Name == "testbinding" && assignedID.Spec.AzureIdentityRef.Name == "test-id" {
				testPass = evtRecorder.Validate(&LastEvent{Type: "Normal", Reason: "binding applied",
					Message: fmt.Sprintf("Binding testbinding applied on node test-node for pod test-pod-%s-test-id", podNamespace)})
				break
			}
		}
	}

	if testPass {
		panic(fmt.Sprintf("assigned id to an unsupported scenario for namespaces: pod: %s, binding: %s, id: %s", podNamespace, bindingNamespace, idNamespace))
	}
}
func TestSupportedScenarios(t *testing.T) {

	testPositiveCases(t, "custom-app-ns", "custom-app-ns", "custom-app-ns")
	testPositiveCases(t, "custom-app-ns", "default", "custom-app-ns")
	testPositiveCases(t, "custom-app-ns", "default", "default")
}

func TestNotSupportedScenarios(t *testing.T) {

	testNegativeCases(t, "custom-app1-ns", "custom-app2-ns", "custom-app2-ns")
	testNegativeCases(t, "custom-app1-ns", "custom-app2-ns", "custom-app3-ns")
	testNegativeCases(t, "custom-app1-ns", "default", "custom-app2-ns")
	testNegativeCases(t, "custom-app1-ns", "custom-app2-ns", "default")
	testNegativeCases(t, "default", "custom-app1-ns", "custom-app1-ns")
	testNegativeCases(t, "default", "default", "custom-app1-ns")
	testNegativeCases(t, "default", "custom-app1-ns", "default")
}

// get right identity - can we have multiple identities with same name?

func TestMapMICClient(t *testing.T) {
	micClient := &TestMICClient{}

	idList := make([]aadpodid.AzureIdentity, 0)

	id := new(aadpodid.AzureIdentity)
	id.Name = "test-azure-identity"

	idList = append(idList, *id)

	id.Name = "test-akssvcrg-id"
	idList = append(idList, *id)

	idMap, _ := micClient.ConvertIDListToMap(&idList)

	name := "test-azure-identity"
	count := 3
	if azureID, idPresent := idMap[name]; idPresent {
		if azureID.Name != name {
			panic("id map id value mismatch")
		}
		count = count - 1
	}

	name = "test-akssvcrg-id"
	if azureID, idPresent := idMap[name]; idPresent {
		if azureID.Name != name {
			panic("id map id value mismatch")
		}
		count = count - 1
	}

	name = "test not there"
	if _, idPresent := idMap[name]; idPresent {
		panic("not present found")
	} else {
		count = count - 1
	}
	if count != 0 {
		panic("Test count mismatch")
	}

}

func TestSimpleMICClient(t *testing.T) {

	exit := make(<-chan struct{}, 0)
	eventCh := make(chan aadpodid.EventType, 100)
	cloudClient := cp.NewTestCloudClient(config.AzureConfig{})
	crdClient := crd.NewTestCrdClient(nil)
	podClient := pod.NewTestPodClient()
	nodeClient := NewTestNodeClient()
	var evtRecorder TestEventRecorder
	evtRecorder.lastEvent = new(LastEvent)

	micClient := NewMICClient(eventCh, cloudClient, crdClient, podClient, nodeClient, &evtRecorder)

	crdClient.CreateId("test-id", "default", aadpodid.UserAssignedMSI, "test-user-msi-resourceid", "test-user-msi-clientid", nil, "", "", "")
	crdClient.CreateBinding("testbinding", "default", "test-id", "test-select")

	nodeClient.AddNode("test-node")
	podClient.AddPod("test-pod", "default", "test-node", "test-select")
	podClient.GetPods()

	eventCh <- aadpodid.PodCreated
	go micClient.Sync(exit)
	time.Sleep(2 * time.Second)
	testPass := false
	listAssignedIDs, err := crdClient.ListAssignedIDs()
	if err != nil {
		glog.Error(err)
		panic("list assigned failed")
	}
	if listAssignedIDs != nil {
		for _, assignedID := range *listAssignedIDs {
			if assignedID.Spec.Pod == "test-pod" && assignedID.Spec.PodNamespace == "default" && assignedID.Spec.NodeName == "test-node" &&
				assignedID.Spec.AzureBindingRef.Name == "testbinding" && assignedID.Spec.AzureIdentityRef.Name == "test-id" {
				testPass = evtRecorder.Validate(&LastEvent{Type: "Normal", Reason: "binding applied",
					Message: "Binding testbinding applied on node test-node for pod test-pod-default-test-id"})
				if !testPass {
					panic("event mismatch")
				}
				break
			}
		}
	}

	if !testPass {
		panic("assigned id mismatch")
	}

	//Test2: Remove assigned id event test
	podClient.DeletePod("test-pod", "default")

	eventCh <- aadpodid.PodDeleted
	time.Sleep(5 * time.Second)
	testPass = false
	listAssignedIDs, err = crdClient.ListAssignedIDs()
	if err != nil {
		glog.Error(err)
		panic("list assigned failed")
	}
	testPass = evtRecorder.Validate(&LastEvent{Type: "Normal", Reason: "binding removed",
		Message: "Binding testbinding removed from node test-node for pod test-pod"})

	if !testPass {
		panic("event mismatch")
	}

	// Test3: Error from cloud provider event test
	err = errors.New("error returned from cloud provider")
	cloudClient.SetError(err)

	podClient.AddPod("test-pod", "default", "test-node", "test-select")
	eventCh <- aadpodid.PodCreated
	time.Sleep(2 * time.Second)

	testPass = evtRecorder.Validate(&LastEvent{Type: "Warning", Reason: "binding apply error",
		Message: "Applying binding testbinding node test-node for pod test-pod-default-test-id resulted in error error returned from cloud provider"})

	if !testPass {
		panic("event mismatch")
	}

	// Test4: Removal error event test
	//Reset the state to add the id.
	cloudClient.UnSetError()

	//podClient.AddPod("test-pod", "default", "test-node", "test-select")
	eventCh <- aadpodid.PodCreated
	time.Sleep(5 * time.Second)

	err = errors.New("remove error returned from cloud provider")
	cloudClient.SetError(err)

	podClient.DeletePod("test-pod", "default")
	eventCh <- aadpodid.PodDeleted
	time.Sleep(5 * time.Second)

	testPass = evtRecorder.Validate(&LastEvent{Type: "Warning", Reason: "binding remove error",
		Message: "Binding testbinding removal from node test-node for pod test-pod resulted in error remove error returned from cloud provider"})

	if !testPass {
		panic("event mismatch")
	}
}
