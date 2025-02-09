package gitlab

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/xanzy/go-gitlab"
)

func resourceGitlabProjectShareGroup() *schema.Resource {
	acceptedAccessLevels := []string{"guest", "reporter", "developer", "maintainer"}

	return &schema.Resource{
		CreateContext: resourceGitlabProjectShareGroupCreate,
		ReadContext:   resourceGitlabProjectShareGroupRead,
		DeleteContext: resourceGitlabProjectShareGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"group_id": {
				Type:     schema.TypeInt,
				ForceNew: true,
				Required: true,
			},
			"access_level": {
				Type:             schema.TypeString,
				ValidateDiagFunc: validateValueFunc(acceptedAccessLevels),
				ForceNew:         true,
				Required:         true,
			},
		},
	}
}

func resourceGitlabProjectShareGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)

	groupId := d.Get("group_id").(int)
	projectId := d.Get("project_id").(string)
	accessLevelId := accessLevelID[d.Get("access_level").(string)]

	options := &gitlab.ShareWithGroupOptions{
		GroupID:     &groupId,
		GroupAccess: &accessLevelId,
	}
	log.Printf("[DEBUG] create gitlab project membership for %d in %s", options.GroupID, projectId)

	_, err := client.Projects.ShareProjectWithGroup(projectId, options, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	groupIdString := strconv.Itoa(groupId)
	d.SetId(buildTwoPartID(&projectId, &groupIdString))
	return resourceGitlabProjectShareGroupRead(ctx, d, meta)
}

func resourceGitlabProjectShareGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	id := d.Id()
	log.Printf("[DEBUG] read gitlab project projectMember %s", id)

	projectId, groupId, err := projectIdAndGroupIdFromId(id)
	if err != nil {
		return diag.FromErr(err)
	}

	projectInformation, _, err := client.Projects.GetProject(projectId, nil, gitlab.WithContext(ctx))
	if err != nil {
		if is404(err) {
			log.Printf("[DEBUG] failed to read gitlab project %s: %s", id, err)
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	for _, v := range projectInformation.SharedWithGroups {
		if groupId == v.GroupID {
			resourceGitlabProjectShareGroupSetToState(d, v, &projectId)
		}
	}

	return nil
}

func projectIdAndGroupIdFromId(id string) (string, int, error) {
	projectId, groupIdString, err := parseTwoPartID(id)
	if err != nil {
		return "", 0, fmt.Errorf("Error parsing ID: %s", id)
	}

	groupId, err := strconv.Atoi(groupIdString)
	if err != nil {
		return "", 0, fmt.Errorf("Can not determine group id: %v", id)
	}

	return projectId, groupId, nil
}

func resourceGitlabProjectShareGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)

	id := d.Id()
	projectId, groupId, err := projectIdAndGroupIdFromId(id)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] Delete gitlab project membership %v for %s", groupId, projectId)

	_, err = client.Projects.DeleteSharedProjectFromGroup(projectId, groupId, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceGitlabProjectShareGroupSetToState(d *schema.ResourceData, group struct {
	GroupID          int    "json:\"group_id\""
	GroupName        string "json:\"group_name\""
	GroupAccessLevel int    "json:\"group_access_level\""
}, projectId *string) {

	//This cast is needed due to an inconsistency in the upstream API
	//GroupAccessLevel is returned as an int but the map we lookup is sorted by the int alias AccessLevelValue
	convertedAccessLevel := gitlab.AccessLevelValue(group.GroupAccessLevel)

	d.Set("project_id", projectId)
	d.Set("group_id", group.GroupID)
	d.Set("access_level", accessLevel[convertedAccessLevel])

	groupId := strconv.Itoa(group.GroupID)
	d.SetId(buildTwoPartID(projectId, &groupId))
}
