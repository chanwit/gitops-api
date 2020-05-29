package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chanwit/gitops-api/secretwriter"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

func run(dir string, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	return cmd.Run()
}

const GitOpsManagedCluster = "gitops-managed-cluster"

func GetWorkflowStatus(request *StatusRequest) (*string, *string, *string, []RunStatus, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: request.GitHubToken})
	authClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(authClient)
	// latest
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(context.Background(), request.TargetOrg, request.TargetRepo, &github.ListOptions{
		Page:    0,
		PerPage: 1,
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if *runs.TotalCount > 0 {
		run := runs.WorkflowRuns[0]
		jobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), request.TargetOrg, request.TargetRepo, *run.ID, &github.ListWorkflowJobsOptions{})
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if *jobs.TotalCount > 0 {
			job := jobs.Jobs[0]
			runStatuses := []RunStatus{}
			for _, step := range job.Steps {
				if step == nil {
					continue
				}
				runStatuses = append(runStatuses, RunStatus{
					Status:     step.Status,
					Message:    step.Name,
					Conclusion: step.Conclusion,
				})
			}
			return run.HTMLURL, job.Status, job.Conclusion, runStatuses, nil
		}
	}
	return nil, nil, nil, nil, errors.New("No data")
}

func cloneClusterTemplate(request *ClusterTemplateCloneRequest) error {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: request.GitHubToken})
	authClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(authClient)

	dir, err := ioutil.TempDir("/tmp", "gitops-")
	if err != nil {
		return err
	}
	templateDir := filepath.Join(dir, "template")

	log.Println("Git Clone ...")
	if err := run(dir, "git", "clone",
		fmt.Sprintf("https://%s:%s@github.com/%s",
			request.GitHubUser,
			request.GitHubToken,
			request.TemplateRepository),
		"template"); err != nil {
		return err
	}
	defer os.RemoveAll(templateDir)

	newRepo := &github.Repository{
		Name:        github.String(request.TargetRepo),
		Private:     github.Bool(true),
		Description: github.String(request.TargetRepo + " repo"),
		Topics:      []string{GitOpsManagedCluster},
	}

	targetOrgOrUser := ""
	if request.GitHubUser != request.TargetOrg {
		targetOrgOrUser = request.TargetOrg
	}

	log.Println("Create new repo ...")
	repo, _, err := client.Repositories.Create(context.Background(), targetOrgOrUser, newRepo)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Printf("Successfully created new repo: %v\n", repo.GetName())

	topics, _, err := client.Repositories.ReplaceAllTopics(context.Background(), request.TargetOrg, request.TargetRepo, []string{GitOpsManagedCluster})
	if err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}
	log.Printf("Successfully set topics to: %v\n", topics)

	cloneUrl := repo.GetCloneURL()

	fmt.Printf("url = %s\n", cloneUrl)
	u, err := url.Parse(cloneUrl)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	u.User = url.UserPassword(request.GitHubUser, request.GitHubToken)

	log.Println("Add new fork ...")
	if err := run(templateDir, "git", "remote", "add", "fork", u.String()); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Println("Replace cluster name ...")
	if err := replaceClusterName(templateDir, request.TargetRepo); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Println("Add secret awsAccessKeyId ...")
	sw := secretwriter.New(client)
	if _, err := sw.Write(
		request.TargetOrg, request.TargetRepo,
		"awsAccessKeyId", []byte(request.Secrets["awsAccessKeyId"])); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Println("Add secret awsSecretAccessKey ...")
	if _, err := sw.Write(
		request.TargetOrg, request.TargetRepo,
		"awsSecretAccessKey", []byte(request.Secrets["awsSecretAccessKey"])); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Println("Add secret githubToken ...")
	if _, err := sw.Write(
		request.TargetOrg, request.TargetRepo,
		"githubToken", []byte(request.GitHubToken)); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	log.Println("Push to new fork")
	// this push will trigger the workflow
	if err := run(templateDir, "git", "push", "fork", "master"); err != nil {
		log.Printf("Error: %v\n", err)
		return err
	}

	return nil
}

func replaceClusterName(templateDir string, clusterName string) error {
	if err := run(templateDir, "yq", "write", "--inplace", "cluster.yaml", "spec.state", "absent"); err != nil {
		return errors.Wrap(err, "Failed replacing spec.state to absent")
	}

	if err := run(templateDir, "yq", "write", "--inplace", "cluster.yaml", "spec.template.metadata.name", clusterName); err != nil {
		return errors.Wrapf(err, "Failed replacing spec.template.metadata.name to %s", clusterName)
	}

	if err := run(templateDir, "git", "diff", "--exit-code"); err == nil {
		// if err == 1, dirty
		// if err == 0, clean
		return errors.New("No code change. Nothing to do.")
	}

	if err := run(templateDir, "git", "commit", "-am", fmt.Sprintf("set state to absent and change name to %s", clusterName)); err != nil {
		return errors.Wrap(err, "Failed git commit")
	}

	return nil
}

func changeClusterState(request *ClusterStateApplyRequest) error {
	dir, err := ioutil.TempDir("/tmp", "gitops-")
	if err != nil {
		return err
	}
	templateDir := filepath.Join(dir, "template")

	if err := run(dir, "git", "clone",
		fmt.Sprintf("https://%s:%s@github.com/%s/%s",
			request.GitHubUser,
			request.GitHubToken,
			request.TargetOrg,
			request.TargetRepo),
		"template"); err != nil {
		return err
	}
	defer os.RemoveAll(templateDir)

	if err := run(templateDir, "yq", "write", "--inplace", "cluster.yaml", "spec.state", request.ClusterState); err != nil {
		return errors.Wrapf(err, "Failed replacing spec.state to %q", request.ClusterState)
	}

	if err := run(templateDir, "git", "diff", "--exit-code"); err == nil {
		// if err == 1, dirty
		// if err == 0, clean
		return errors.New("No code change. Nothing to do.")
	}

	if err := run(templateDir, "git", "commit", "-am", fmt.Sprintf("Changed cluster state to %s", request.ClusterState)); err != nil {
		return errors.Wrapf(err, "Failed replacing spec.state to %q", request.ClusterState)
	}

	if err := run(templateDir, "git", "push", "origin", "master"); err != nil {
		return errors.Wrap(err, "Push failed")
	}

	return nil
}

func applyProfiles(request *ProfileApplyRequest) error {
	dir, err := ioutil.TempDir("/tmp", "gitops-")
	if err != nil {
		return err
	}
	templateDir := filepath.Join(dir, "template")

	if err := run(dir, "git", "clone",
		fmt.Sprintf("https://%s:%s@github.com/%s/%s",
			request.GitHubUser,
			request.GitHubToken,
			request.TargetOrg,
			request.TargetRepo),
		"template"); err != nil {
		return err
	}
	defer os.RemoveAll(templateDir)

	// clear profiles
	if err := run(templateDir, "yq", "write", "--inplace", "cluster.yaml", "spec.profiles", "[]"); err != nil {
		return errors.Wrapf(err, "Failed to clear spec.profiles")
	}

	for _, profile := range request.Profiles {
		if err := run(templateDir, "yq", "--prettyPrint", "write", "--inplace", "cluster.yaml", "spec.profiles[+]", profile); err != nil {
			return errors.Wrapf(err, "Failed adding spec.profiles[+] to %q", profile)
		}
	}

	if err := run(templateDir, "git", "diff", "--exit-code"); err == nil {
		// if err == 1, dirty
		// if err == 0, clean
		return errors.New("No code change. Nothing to do.")
	}

	if err := run(templateDir, "git", "commit", "-am", "Changed profiles"); err != nil {
		return errors.Wrap(err, "Failed to commit spec.profiles")
	}

	if err := run(templateDir, "git", "push", "origin", "master"); err != nil {
		return errors.Wrap(err, "Push failed")
	}

	return nil
}

func main() {
	r := gin.Default()
	r.Use(cors.Default())
	r.POST("/api/cluster/clone-from-template", func(c *gin.Context) {
		log.Println("Entering /api/cluster/clone-from-template ... ")
		req := &ClusterTemplateCloneRequest{}
		if err := c.BindJSON(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
			return
		}
		log.Printf("Bind JSON to req: OK ..: %#v", req)

		if err := cloneClusterTemplate(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Template cloning failed: %v", err),
			})
			return
		}
		log.Println("Clone cluster OK")

		c.JSON(200, gin.H{
			"result": "Template cloning successfully",
		})
	})

	r.POST("/api/cluster/state", func(c *gin.Context) {
		req := &ClusterStateApplyRequest{}
		if err := c.BindJSON(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
			return
		}
		if err := changeClusterState(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Cluster state change failed: %v", err),
			})
			return
		}

		c.JSON(200, gin.H{
			"result": fmt.Sprintf("Cluster desired state changed to %s", req.ClusterState),
		})
	})

	r.POST("/api/cluster/profiles", func(c *gin.Context) {
		req := &ProfileApplyRequest{}
		if err := c.BindJSON(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
			return
		}
		if err := applyProfiles(req); err != nil {
			c.JSON(400, gin.H{
				"error": "Cluster profiles apply failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"result": "Profiles applied",
		})
	})

	r.POST("/api/cluster/run-status", func(c *gin.Context) {
		req := &StatusRequest{}
		if err := c.BindJSON(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
			return
		}
		link, jobStatus, jobConclusion, statuses, err := GetWorkflowStatus(req)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Fail to get cluster status",
			})
			return
		}
		c.JSON(200, gin.H{
			"status": jobStatus,
			"conclusion": jobConclusion,
			"result": statuses,
			"link":   link,
		})
	})

	r.POST("/api/clusters", func(c *gin.Context) {
		req := &ListClusterRequest{}
		if err := c.BindJSON(req); err != nil {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
			return
		}
		clusterStatus, err := listClusters(req)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Fail to list clusters and their status",
			})
			return
		}
		c.JSON(200, gin.H{
			"result": clusterStatus,
		})
	})

	r.Run()
}

func listClusters(request *ListClusterRequest) ([]*ClusterStatus, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: request.GitHubToken})
	authClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(authClient)
	searchResult, _, err := client.Search.Repositories(context.Background(), "topic:"+GitOpsManagedCluster, &github.SearchOptions{
		Sort:  "pushed",
		Order: "desc",
		ListOptions: github.ListOptions{
			0, 50,
		},
	})

	if err != nil {
		return nil, err
	}
	fmt.Println(searchResult.GetTotal())

	result := []*ClusterStatus{}
	for _, repo := range searchResult.Repositories {
		for _, t := range repo.Topics {
			if t == GitOpsManagedCluster {
				result = append(result, &ClusterStatus{
					Name:      *repo.FullName,
					Status:    "",
					Link:      "",
					RunStatus: nil,
				})
			}
		}
	}

	for _, c := range result {
		parts := strings.SplitN(c.Name, "/", 2)
		link, status, conclusion, runStatuses, err := GetWorkflowStatus(&StatusRequest{
			TargetOrg:   parts[0],
			TargetRepo:  parts[1],
			GitHubUser:  request.GitHubUser,
			GitHubToken: request.GitHubToken,
		})
		if err != nil {
			return nil, err
		}
		c.Status = *status
		c.Conclusion = *conclusion
		c.RunStatus = runStatuses
		c.Link = *link
	}

	return result, err
}
