package controller

import (
	pb "github.com/uber/tango/tangopb"
)

func (c *controller) GetChangedServices(request *pb.GetChangedServicesRequest, stream pb.TangoServiceGetChangedServicesYARPCServer) error {
	return nil
}
