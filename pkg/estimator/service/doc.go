package service

//go:generate protoc --gogo_opt=paths=source_relative -I . -I ../../.. -I ../../../vendor --gogo_out=plugins=grpc:. --gogo_opt=Mpkg/estimator/pb/generated.proto=github.com/karmada-io/karmada/pkg/estimator/pb service.proto
//go:generate mockery --name=EstimatorClient --inpackage
