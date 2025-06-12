package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
)

func main() {
	var (
		subscriptionID        string
		resourceGroup         string
		location              string
		userImageName         string
		managedDiskName       string
		galleryName           string
		imageDefinitionName   string
		imageVersionName      string
		communityGalleryImage string
		targetRegionsStr      string
	)

	flag.StringVar(&subscriptionID, "subscription-id", os.Getenv("AZURE_SUBSCRIPTION_ID"), "Azure subscription ID")
	flag.StringVar(&resourceGroup, "resource-group", os.Getenv("AZURE_RESOURCE_GROUP"), "Resource group name")
	flag.StringVar(&location, "location", os.Getenv("AZURE_REGION"), "Azure region")
	flag.StringVar(&userImageName, "user-image-name", "from-community-gallery-user-image", "Name of the temporary managed image")
	flag.StringVar(&managedDiskName, "managed-disk-name", "from-community-gallery", "Name of the temporary managed disk")
	flag.StringVar(&galleryName, "gallery-name", "my_gallery", "Shared Image Gallery name")
	flag.StringVar(&imageDefinitionName, "definition-name", "my-def-cvm", "Image definition name")
	flag.StringVar(&imageVersionName, "version-name", "0.0.1", "Image version name")
	flag.StringVar(&communityGalleryImage, "community-image-id", "", "Community gallery image version resource ID")
	flag.StringVar(&targetRegionsStr, "target-regions", "westeurope,koreasouth,eastus", "Comma separated target regions")

	flag.Parse()

	if subscriptionID == "" || resourceGroup == "" || location == "" || communityGalleryImage == "" {
		fmt.Fprintln(os.Stderr, "subscription-id, resource-group, location and community-image-id are required")
		os.Exit(1)
	}

	regions := strings.Split(targetRegionsStr, ",")
	var targets []*armcompute.TargetRegion
	for _, r := range regions {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		targets = append(targets, &armcompute.TargetRegion{Name: to.Ptr(r)})
	}

	ctx := context.Background()
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("creating credential: %v", err)
	}

	diskClient, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating disks client: %v", err)
	}

	log.Printf("creating managed disk %s from community image", managedDiskName)
	diskPoller, err := diskClient.BeginCreateOrUpdate(ctx, resourceGroup, managedDiskName, armcompute.Disk{
		Location: to.Ptr(location),
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:          to.Ptr(armcompute.DiskCreateOptionFromImage),
				GalleryImageReference: &armcompute.GalleryImageReference{CommunityGalleryImageID: to.Ptr(communityGalleryImage)},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating disk: %v", err)
	}
	if _, err = diskPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for disk: %v", err)
	}

	diskID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", subscriptionID, resourceGroup, managedDiskName)

	imagesClient, err := armcompute.NewImagesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating images client: %v", err)
	}

	log.Printf("creating managed image %s", userImageName)
	imgPoller, err := imagesClient.BeginCreateOrUpdate(ctx, resourceGroup, userImageName, armcompute.Image{
		Location: to.Ptr(location),
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:      to.Ptr(armcompute.OperatingSystemTypesLinux),
					OSState:     to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
					ManagedDisk: &armcompute.SubResource{ID: to.Ptr(diskID)},
				},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating managed image: %v", err)
	}
	if _, err = imgPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for managed image: %v", err)
	}

	imageID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s", subscriptionID, resourceGroup, userImageName)

	galleriesClient, err := armcompute.NewGalleriesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating galleries client: %v", err)
	}
	galPoller, err := galleriesClient.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, armcompute.Gallery{Location: to.Ptr(location)}, nil)
	if err != nil {
		log.Fatalf("creating gallery: %v", err)
	}
	if _, err = galPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for gallery: %v", err)
	}

	imgDefClient, err := armcompute.NewGalleryImagesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating gallery images client: %v", err)
	}
	imgDefPoller, err := imgDefClient.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, imageDefinitionName, armcompute.GalleryImage{
		Location: to.Ptr(location),
		Properties: &armcompute.GalleryImageProperties{
			OSType:           to.Ptr(armcompute.OperatingSystemTypesLinux),
			OSState:          to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: to.Ptr("cvm-publisher"),
				Offer:     to.Ptr("cvm-offer"),
				SKU:       to.Ptr("cvm-sku"),
			},
			Features: []*armcompute.GalleryImageFeature{{
				Name:  to.Ptr("SecurityType"),
				Value: to.Ptr("ConfidentialVmSupported"),
			}},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating image definition: %v", err)
	}
	if _, err = imgDefPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for image definition: %v", err)
	}

	versionClient, err := armcompute.NewGalleryImageVersionsClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating gallery image versions client: %v", err)
	}
	verPoller, err := versionClient.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, imageDefinitionName, imageVersionName, armcompute.GalleryImageVersion{
		Location: to.Ptr(location),
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{TargetRegions: targets},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				Source: &armcompute.GalleryArtifactVersionFullSource{ID: to.Ptr(imageID)},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating image version: %v", err)
	}
	if _, err = verPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for image version: %v", err)
	}

	log.Printf("deleting temporary managed image %s", userImageName)
	delImgPoller, err := imagesClient.BeginDelete(ctx, resourceGroup, userImageName, nil)
	if err == nil {
		_, _ = delImgPoller.PollUntilDone(ctx, nil)
	}

	log.Printf("deleting temporary managed disk %s", managedDiskName)
	delDiskPoller, err := diskClient.BeginDelete(ctx, resourceGroup, managedDiskName, nil)
	if err == nil {
		_, _ = delDiskPoller.PollUntilDone(ctx, nil)
	}

	log.Printf("image version %s/%s/%s created successfully", galleryName, imageDefinitionName, imageVersionName)
}
