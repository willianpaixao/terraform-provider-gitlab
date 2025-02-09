package gitlab

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	gitlab "github.com/xanzy/go-gitlab"
)

func resourceGitlabGroupLabel() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitlabGroupLabelCreate,
		ReadContext:   resourceGitlabGroupLabelRead,
		UpdateContext: resourceGitlabGroupLabelUpdate,
		DeleteContext: resourceGitlabGroupLabelDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceGitlabGroupLabelImporter,
		},

		Schema: map[string]*schema.Schema{
			"group": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"color": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceGitlabGroupLabelCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	group := d.Get("group").(string)
	options := &gitlab.CreateGroupLabelOptions{
		Name:  gitlab.String(d.Get("name").(string)),
		Color: gitlab.String(d.Get("color").(string)),
	}

	if v, ok := d.GetOk("description"); ok {
		options.Description = gitlab.String(v.(string))
	}

	log.Printf("[DEBUG] create gitlab group label %s", *options.Name)

	label, _, err := client.GroupLabels.CreateGroupLabel(group, options, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(label.Name)

	return resourceGitlabGroupLabelRead(ctx, d, meta)
}

func resourceGitlabGroupLabelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	group := d.Get("group").(string)
	labelName := d.Id()
	log.Printf("[DEBUG] read gitlab group label %s/%s", group, labelName)

	page := 1
	labelsLen := 0
	for page == 1 || labelsLen != 0 {
		labels, _, err := client.GroupLabels.ListGroupLabels(group, &gitlab.ListGroupLabelsOptions{Page: page}, gitlab.WithContext(ctx))
		if err != nil {
			return diag.FromErr(err)
		}
		for _, label := range labels {
			if label.Name == labelName {
				d.Set("description", label.Description)
				d.Set("color", label.Color)
				d.Set("name", label.Name)
				return nil
			}
		}
		labelsLen = len(labels)
		page = page + 1
	}

	log.Printf("[DEBUG] failed to read gitlab label %s/%s", group, labelName)
	d.SetId("")
	return nil
}

func resourceGitlabGroupLabelUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	group := d.Get("group").(string)
	options := &gitlab.UpdateGroupLabelOptions{
		Name:  gitlab.String(d.Get("name").(string)),
		Color: gitlab.String(d.Get("color").(string)),
	}

	if d.HasChange("description") {
		options.Description = gitlab.String(d.Get("description").(string))
	}

	log.Printf("[DEBUG] update gitlab group label %s", d.Id())

	_, _, err := client.GroupLabels.UpdateGroupLabel(group, options, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceGitlabGroupLabelRead(ctx, d, meta)
}

func resourceGitlabGroupLabelDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	group := d.Get("group").(string)
	log.Printf("[DEBUG] Delete gitlab group label %s", d.Id())
	options := &gitlab.DeleteGroupLabelOptions{
		Name: gitlab.String(d.Id()),
	}

	_, err := client.GroupLabels.DeleteGroupLabel(group, options, gitlab.WithContext(ctx))
	return diag.FromErr(err)
}

func resourceGitlabGroupLabelImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*gitlab.Client)
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid label id (should be <group ID>.<label name>): %s", d.Id())
	}

	d.SetId(parts[1])
	group, _, err := client.Groups.GetGroup(parts[0], nil)
	if err != nil {
		return nil, err
	}

	if err := d.Set("group", strconv.Itoa(group.ID)); err != nil {
		return nil, err
	}

	diagnostic := resourceGitlabGroupLabelRead(ctx, d, meta)
	if diagnostic.HasError() {
		return nil, fmt.Errorf("failed to read group label %s: %s", d.Id(), diagnostic[0].Summary)
	}

	return []*schema.ResourceData{d}, nil
}
