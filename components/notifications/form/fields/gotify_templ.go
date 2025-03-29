// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.833
package fields

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"github.com/starfleetcptn/gomft/components/notifications/form/utils"
	"github.com/starfleetcptn/gomft/components/notifications/types"
)

func GotifyFields(data types.NotificationFormData) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div id=\"gotify_fields\" class=\"hidden notification-fields\"><div class=\"mb-6\"><label for=\"gotify_url\" class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Gotify Server URL</label> ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.GotifyURL != "" {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<input type=\"url\" id=\"gotify_url\" name=\"gotify_url\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"https://gotify.example.com\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var2 string
			templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(data.NotificationService.GotifyURL)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 13, Col: 407}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "<input type=\"url\" id=\"gotify_url\" name=\"gotify_url\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"https://gotify.example.com\" value=\"\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "<p class=\"mt-1 text-sm text-gray-500 dark:text-gray-400\">URL of your Gotify server</p></div><div class=\"mb-6\"><label for=\"gotify_token\" class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Application Token</label> ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.GotifyToken != "" {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "<input type=\"text\" id=\"gotify_token\" name=\"gotify_token\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"A-M-XiEQj.zX5d\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var3 string
			templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(data.NotificationService.GotifyToken)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 22, Col: 402}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "<input type=\"text\" id=\"gotify_token\" name=\"gotify_token\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"A-M-XiEQj.zX5d\" value=\"\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "<p class=\"mt-1 text-sm text-gray-500 dark:text-gray-400\">Find this in your Gotify application settings</p></div><div class=\"mb-6\"><label for=\"gotify_priority\" class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Default Priority</label> <select id=\"gotify_priority\" name=\"gotify_priority\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\"><option value=\"0\">Low (0)</option> ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.GotifyPriority != "" && data.NotificationService.GotifyPriority == "5" {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "<option value=\"5\" selected=\"selected\">Normal (5)</option> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "<option value=\"5\">Normal (5)</option> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "<option value=\"8\">High (8)</option></select></div><div class=\"mb-6\"><label for=\"gotify_title_template\" class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Message Title Template</label> ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.GotifyTitleTemplate != "" {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "<input type=\"text\" id=\"gotify_title_template\" name=\"gotify_title_template\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var4 string
			templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(data.NotificationService.GotifyTitleTemplate)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 43, Col: 399}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "<input type=\"text\" id=\"gotify_title_template\" name=\"gotify_title_template\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"{{job.name}} {{job.status}}\" value=\"\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "</div><div class=\"mb-6\"><label for=\"gotify_message_template\" class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Message Body Template</label> <textarea id=\"gotify_message_template\" name=\"gotify_message_template\" rows=\"4\" class=\"bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500\" placeholder=\"Job &#39;{{job.name}}&#39; {{job.status}} at {{job.completed_at}}. {{job.file_count}} files transferred ({{job.transfer_bytes}} bytes).\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.GotifyMessageTemplate != "" {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 17, "data.NotificationService.GotifyMessageTemplate")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 18, "</textarea><p class=\"mt-1 text-sm text-gray-500 dark:text-gray-400\">Use placeholders for dynamic values. Available variables: job.*, instance.*, timestamp, notification.*</p></div><div class=\"mb-6\"><label class=\"block mb-2 text-sm font-medium text-gray-900 dark:text-white\">Event Triggers</label><div class=\"space-y-2\"><div class=\"flex items-center\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.EventTriggers != nil {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 19, "<input id=\"gotify_trigger_job_start\" name=\"trigger_job_start\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\" checked=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var5 string
			templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(utils.BoolToString(utils.Contains(data.NotificationService.EventTriggers, "job_start")))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 67, Col: 361}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 20, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 21, "<input id=\"gotify_trigger_job_start\" name=\"trigger_job_start\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 22, "<label for=\"gotify_trigger_job_start\" class=\"ml-2 text-sm font-medium text-gray-900 dark:text-white\">Job Start</label></div><div class=\"flex items-center\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.EventTriggers != nil {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 23, "<input id=\"gotify_trigger_job_complete\" name=\"trigger_job_complete\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\" checked=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var6 string
			templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(utils.BoolToString(utils.Contains(data.NotificationService.EventTriggers, "job_complete")))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 75, Col: 370}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 24, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 25, "<input id=\"gotify_trigger_job_complete\" name=\"trigger_job_complete\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 26, "<label for=\"gotify_trigger_job_complete\" class=\"ml-2 text-sm font-medium text-gray-900 dark:text-white\">Job Complete</label></div><div class=\"flex items-center\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if data.NotificationService.EventTriggers != nil {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 27, "<input id=\"gotify_trigger_job_error\" name=\"trigger_job_error\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\" checked=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var7 string
			templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(utils.BoolToString(utils.Contains(data.NotificationService.EventTriggers, "job_error")))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/form/fields/gotify.templ`, Line: 83, Col: 361}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 28, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 29, "<input id=\"gotify_trigger_job_error\" name=\"trigger_job_error\" type=\"checkbox\" class=\"w-4 h-4 border border-gray-300 rounded bg-gray-50 focus:ring-3 focus:ring-blue-300 dark:bg-gray-700 dark:border-gray-600 dark:focus:ring-blue-600 dark:ring-offset-gray-800\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 30, "<label for=\"gotify_trigger_job_error\" class=\"ml-2 text-sm font-medium text-gray-900 dark:text-white\">Job Error</label></div></div></div><!-- Test notification button for Gotify --><div class=\"mb-6 p-4 bg-gray-50 border border-gray-200 rounded-lg dark:bg-gray-700 dark:border-gray-600\"><div class=\"flex items-center justify-between mb-2\"><h4 class=\"text-base font-medium text-gray-900 dark:text-white\">Test Configuration</h4><button type=\"button\" id=\"test-gotify-btn\" hx-post=\"/admin/settings/notifications/test\" hx-trigger=\"click\" hx-target=\"#test-notification-result\" hx-swap=\"outerHTML\" class=\"px-3 py-2 text-xs font-medium text-white bg-blue-700 hover:bg-blue-800 focus:ring-4 focus:ring-blue-300 rounded-lg dark:bg-blue-600 dark:hover:bg-blue-700 focus:outline-none dark:focus:ring-blue-800\"><i class=\"fas fa-paper-plane mr-1\"></i> Send Test Notification</button></div><p class=\"text-sm text-gray-500 dark:text-gray-400\">Send a test notification to verify your Gotify configuration works correctly before saving.</p><div id=\"test-notification-result\" class=\"mt-3 hidden\"><!-- Result will be shown here --></div></div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
