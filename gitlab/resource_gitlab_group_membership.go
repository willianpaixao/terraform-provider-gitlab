package gitlab

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/xanzy/go-gitlab"
)

func resourceGitlabGroupMembership() *schema.Resource {
	acceptedAccessLevels := make([]string, 0, len(accessLevelID))
	for k := range accessLevelID {
		acceptedAccessLevels = append(acceptedAccessLevels, k)
	}
	return &schema.Resource{
		CreateContext: resourceGitlabGroupMembershipCreate,
		ReadContext:   resourceGitlabGroupMembershipRead,
		UpdateContext: resourceGitlabGroupMembershipUpdate,
		DeleteContext: resourceGitlabGroupMembershipDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"group_id": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"user_id": {
				Type:     schema.TypeInt,
				ForceNew: true,
				Required: true,
			},
			"access_level": {
				Type:             schema.TypeString,
				ValidateDiagFunc: validateValueFunc(acceptedAccessLevels),
				Required:         true,
			},
			"expires_at": {
				Type:         schema.TypeString, // Format YYYY-MM-DD
				ValidateFunc: validateDateFunc,
				Optional:     true,
			},
		},
	}
}

func resourceGitlabGroupMembershipCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)

	userId := d.Get("user_id").(int)
	groupId := d.Get("group_id").(string)
	expiresAt := d.Get("expires_at").(string)
	accessLevelId := accessLevelID[d.Get("access_level").(string)]

	options := &gitlab.AddGroupMemberOptions{
		UserID:      &userId,
		AccessLevel: &accessLevelId,
		ExpiresAt:   &expiresAt,
	}
	log.Printf("[DEBUG] create gitlab group groupMember for %d in %s", options.UserID, groupId)

	groupMember, _, err := client.GroupMembers.AddGroupMember(groupId, options, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	userIdString := strconv.Itoa(groupMember.ID)
	d.SetId(buildTwoPartID(&groupId, &userIdString))
	return resourceGitlabGroupMembershipRead(ctx, d, meta)
}

func resourceGitlabGroupMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	id := d.Id()
	log.Printf("[DEBUG] read gitlab group groupMember %s", id)

	groupId, userId, err := groupIdAndUserIdFromId(id)
	if err != nil {
		return diag.FromErr(err)
	}

	groupMember, _, err := client.GroupMembers.GetGroupMember(groupId, userId, gitlab.WithContext(ctx))
	if err != nil {
		if is404(err) {
			log.Printf("[DEBUG] gitlab group membership for %s not found so removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	resourceGitlabGroupMembershipSetToState(d, groupMember, &groupId)
	return nil
}

func groupIdAndUserIdFromId(id string) (string, int, error) {
	groupId, userIdString, err := parseTwoPartID(id)
	userId, e := strconv.Atoi(userIdString)
	if err != nil {
		e = err
	}
	if e != nil {
		log.Printf("[WARN] cannot get group member id from input: %v", id)
	}
	return groupId, userId, e
}

func resourceGitlabGroupMembershipUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)

	userId := d.Get("user_id").(int)
	groupId := d.Get("group_id").(string)
	expiresAt := d.Get("expires_at").(string)
	accessLevelId := accessLevelID[strings.ToLower(d.Get("access_level").(string))]

	options := gitlab.EditGroupMemberOptions{
		AccessLevel: &accessLevelId,
		ExpiresAt:   &expiresAt,
	}
	log.Printf("[DEBUG] update gitlab group membership %v for %s", userId, groupId)

	_, _, err := client.GroupMembers.EditGroupMember(groupId, userId, &options, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceGitlabGroupMembershipRead(ctx, d, meta)
}

func resourceGitlabGroupMembershipDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)

	id := d.Id()
	groupId, userId, err := groupIdAndUserIdFromId(id)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] Delete gitlab group membership %v for %s", userId, groupId)

	_, err = client.GroupMembers.RemoveGroupMember(groupId, userId, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceGitlabGroupMembershipSetToState(d *schema.ResourceData, groupMember *gitlab.GroupMember, groupId *string) {

	d.Set("group_id", groupId)
	d.Set("user_id", groupMember.ID)
	d.Set("access_level", accessLevel[groupMember.AccessLevel])
	if groupMember.ExpiresAt != nil {
		d.Set("expires_at", groupMember.ExpiresAt.String())
	}
	userId := strconv.Itoa(groupMember.ID)
	d.SetId(buildTwoPartID(groupId, &userId))
}
