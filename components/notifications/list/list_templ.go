// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.833
package list

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"context"
	"fmt"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/components/notifications/dialog"
	"github.com/starfleetcptn/gomft/components/notifications/types"
)

// List renders the notification services list page.
func List(ctx context.Context, data types.SettingsNotificationsData) templ.Component {
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
		templ_7745c5c3_Var2 := templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
			templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
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
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<!-- Status and Error Messages (Handled by shared toast component in layout) --> <div id=\"notifications-container\" class=\"notifications-page bg-gray-50 dark:bg-gray-900 min-h-screen\"><div class=\"pb-8 w-full\"><!-- Success Message (hidden, used for HTMX responses/toast trigger) -->")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			if data.SuccessMessage != "" {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<div class=\"hidden success-message\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var3 string
				templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(data.SuccessMessage)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 20, Col: 62}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "</div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "<!-- Error Message (hidden, used for HTMX responses/toast trigger) -->")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			if data.ErrorMessage != "" {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "<div class=\"hidden error-message\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var4 string
				templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(data.ErrorMessage)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 24, Col: 58}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "</div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "<div class=\"mb-6 flex flex-col md:flex-row md:items-center md:justify-between gap-4\"><h1 class=\"text-2xl font-bold text-gray-900 dark:text-white flex items-center\"><i class=\"fas fa-bell w-6 h-6 mr-2 text-blue-500 dark:text-blue-400\"></i> Notification Services</h1><a href=\"/admin/settings/notifications/new\" class=\"flex items-center justify-center text-white bg-blue-700 hover:bg-blue-800 focus:ring-4 focus:ring-blue-300 font-medium rounded-lg px-5 py-2.5 dark:bg-blue-600 dark:hover:bg-blue-700 focus:outline-none dark:focus:ring-blue-800\"><i class=\"fas fa-plus w-4 h-4 mr-2\"></i> Add Notification Service</a></div><!-- List of Notification Services -->")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			if len(data.NotificationServices) == 0 {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "<div class=\"text-center py-8 bg-white dark:bg-gray-800 shadow-md rounded-lg\"><div class=\"inline-flex items-center justify-center w-16 h-16 rounded-full bg-blue-100 dark:bg-blue-900 mb-4\"><i class=\"fas fa-bell text-2xl text-blue-600 dark:text-blue-400\"></i></div><h3 class=\"mb-2 text-lg font-semibold text-gray-900 dark:text-white\">No notification services configured</h3><p class=\"text-gray-500 dark:text-gray-400 mb-4\">Add a notification service to receive alerts for job events.</p><a href=\"/admin/settings/notifications/new\" class=\"inline-flex items-center px-3 py-2 text-sm font-medium text-center text-white bg-blue-700 rounded-lg hover:bg-blue-800 focus:ring-4 focus:outline-none focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800\"><i class=\"fas fa-plus w-4 h-4 mr-2\"></i> Add First Notification Service</a></div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			} else {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "<div class=\"bg-white border border-gray-200 rounded-lg shadow-sm dark:border-gray-700 dark:bg-gray-800 overflow-hidden\"><ul class=\"divide-y divide-gray-200 dark:divide-gray-700\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				for _, service := range data.NotificationServices {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "<li><div class=\"block hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors\"><div class=\"px-4 py-4 sm:px-6\"><div class=\"flex items-center justify-between\"><div class=\"flex items-center\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					if service.Type == "email" {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "<div class=\"w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 dark:bg-blue-900 dark:text-blue-400 mr-3\"><i class=\"fas fa-envelope\"></i></div>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					} else if service.Type == "webhook" {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "<div class=\"w-10 h-10 rounded-full bg-green-100 flex items-center justify-center text-green-600 dark:bg-green-900 dark:text-green-400 mr-3\"><i class=\"fas fa-code\"></i></div>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					} else {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, " <div class=\"w-10 h-10 rounded-full bg-gray-100 flex items-center justify-center text-gray-600 dark:bg-gray-700 dark:text-gray-400 mr-3\"><i class=\"fas fa-bell\"></i></div>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "<div><p class=\"text-sm font-medium text-blue-600 dark:text-blue-400 truncate\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var5 string
					templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(service.Name)
					if templ_7745c5c3_Err != nil {
						return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 75, Col: 29}
					}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "</p><p class=\"text-sm text-gray-500 dark:text-gray-400 mt-1\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var6 string
					templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(service.Description)
					if templ_7745c5c3_Err != nil {
						return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 78, Col: 36}
					}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "</p></div></div><div class=\"ml-2 flex-shrink-0 flex space-x-2\"><a href=\"")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var7 templ.SafeURL = templ.SafeURL(fmt.Sprintf("/admin/settings/notifications/%d/edit", service.ID))
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var7)))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 17, "\" class=\"text-gray-500 bg-white focus:outline-none hover:bg-gray-100 focus:ring-4 focus:ring-gray-200 rounded-lg text-sm p-2 mr-1 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white dark:focus:ring-gray-700\"><i class=\"fas fa-edit\"></i></a><!-- Add notification delete dialog -->")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = dialog.NotificationDialog(
						fmt.Sprintf("delete-notification-dialog-%d", service.ID),
						"Delete Notification Service",
						fmt.Sprintf("Are you sure you want to delete the notification service '%s'? This cannot be undone.", service.Name),
						"text-white bg-red-700 hover:bg-red-800 focus:ring-4 focus:ring-red-300 font-medium rounded-lg text-sm px-5 py-2.5 dark:bg-red-600 dark:hover:bg-red-700 focus:outline-none dark:focus:ring-red-800",
						"Delete",
						"delete",
						service.ID,
						service.Name,
					).Render(ctx, templ_7745c5c3_Buffer)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templ.RenderScriptItems(ctx, templ_7745c5c3_Buffer, templ.ComponentScript{Call: fmt.Sprintf("showModal('delete-notification-dialog-%d')", service.ID)})
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 18, "<button type=\"button\" onclick=\"")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var8 templ.ComponentScript = templ.ComponentScript{Call: fmt.Sprintf("showModal('delete-notification-dialog-%d')", service.ID)}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var8.Call)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 19, "\" class=\"text-red-500 bg-white focus:outline-none hover:bg-gray-100 focus:ring-4 focus:ring-gray-200 rounded-lg text-sm p-2 dark:bg-gray-800 dark:text-red-400 dark:hover:bg-gray-700 dark:hover:text-white dark:focus:ring-gray-700\"><i class=\"fas fa-trash-alt\"></i></button></div></div><div class=\"mt-3 sm:flex sm:justify-between\"><div class=\"sm:flex flex-col md:flex-row gap-2 md:gap-6\"><div class=\"flex items-center\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var9 = []any{"px-2 py-1 text-xs font-medium rounded-full",
						templ.KV("bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300", service.IsEnabled),
						templ.KV("bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300", !service.IsEnabled)}
					templ_7745c5c3_Err = templ.RenderCSSItems(ctx, templ_7745c5c3_Buffer, templ_7745c5c3_Var9...)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 20, "<span class=\"")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var10 string
					templ_7745c5c3_Var10, templ_7745c5c3_Err = templ.JoinStringErrs(templ.CSSClasses(templ_7745c5c3_Var9).String())
					if templ_7745c5c3_Err != nil {
						return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 1, Col: 0}
					}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var10))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 21, "\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					if service.IsEnabled {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 22, "Active")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					} else {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 23, "Disabled")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 24, "</span> <span class=\"ml-2 px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300 rounded-full\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var11 string
					templ_7745c5c3_Var11, templ_7745c5c3_Err = templ.JoinStringErrs(service.Type)
					if templ_7745c5c3_Err != nil {
						return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 124, Col: 29}
					}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var11))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 25, "</span> ")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					if len(service.EventTriggers) > 0 && service.Type == "webhook" {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 26, "<span class=\"ml-2 px-2 py-1 text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300 rounded-full\">")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						var templ_7745c5c3_Var12 string
						templ_7745c5c3_Var12, templ_7745c5c3_Err = templ.JoinStringErrs(fmt.Sprintf("%d triggers", len(service.EventTriggers)))
						if templ_7745c5c3_Err != nil {
							return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 128, Col: 72}
						}
						_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var12))
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 27, "</span> ")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					if service.SuccessCount > 0 || service.FailureCount > 0 {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 28, "<span class=\"ml-2 px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300 rounded-full\">")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						var templ_7745c5c3_Var13 string
						templ_7745c5c3_Var13, templ_7745c5c3_Err = templ.JoinStringErrs(fmt.Sprintf("%d/%d", service.SuccessCount, service.SuccessCount+service.FailureCount))
						if templ_7745c5c3_Err != nil {
							return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 133, Col: 105}
						}
						_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var13))
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 29, "</span>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 30, "</div></div>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					if service.Type == "webhook" {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 31, "<div class=\"mt-2 md:mt-0 flex items-center space-x-4\"><div class=\"text-xs\"><span class=\"text-gray-500 dark:text-gray-400\">Events:</span> <span class=\"ml-1 text-gray-900 dark:text-gray-300\">")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						if len(service.EventTriggers) == 0 {
							templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 32, "None")
							if templ_7745c5c3_Err != nil {
								return templ_7745c5c3_Err
							}
						} else {
							for i, trigger := range service.EventTriggers {
								if i > 0 {
									templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 33, "<span>, </span>")
									if templ_7745c5c3_Err != nil {
										return templ_7745c5c3_Err
									}
								}
								templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 34, " ")
								if templ_7745c5c3_Err != nil {
									return templ_7745c5c3_Err
								}
								var templ_7745c5c3_Var14 string
								templ_7745c5c3_Var14, templ_7745c5c3_Err = templ.JoinStringErrs(trigger)
								if templ_7745c5c3_Err != nil {
									return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 150, Col: 27}
								}
								_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var14))
								if templ_7745c5c3_Err != nil {
									return templ_7745c5c3_Err
								}
							}
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 35, "</span></div><div class=\"text-xs\"><span class=\"text-gray-500 dark:text-gray-400\">Retry:</span> <span class=\"ml-1 text-gray-900 dark:text-gray-300\">")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						if service.RetryPolicy == "" {
							templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 36, "Default")
							if templ_7745c5c3_Err != nil {
								return templ_7745c5c3_Err
							}
						} else {
							var templ_7745c5c3_Var15 string
							templ_7745c5c3_Var15, templ_7745c5c3_Err = templ.JoinStringErrs(service.RetryPolicy)
							if templ_7745c5c3_Err != nil {
								return templ.Error{Err: templ_7745c5c3_Err, FileName: `components/notifications/list/list.templ`, Line: 161, Col: 38}
							}
							_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var15))
							if templ_7745c5c3_Err != nil {
								return templ_7745c5c3_Err
							}
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 37, "</span></div></div>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					} else {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 38, "<div class=\"mt-2 md:mt-0 flex items-center text-sm text-gray-500 dark:text-gray-400\"><i class=\"far fa-clock w-4 h-4 mr-1.5 text-gray-400 dark:text-gray-500\"></i><p>Last sent: ")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						if service.SuccessCount > 0 {
							templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 39, "\"Recently\"")
							if templ_7745c5c3_Err != nil {
								return templ_7745c5c3_Err
							}
						} else {
							templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 40, "\"Never\"")
							if templ_7745c5c3_Err != nil {
								return templ_7745c5c3_Err
							}
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 41, "</p></div>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 42, "</div></div></div></li>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 43, "</ul></div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 44, "<!-- Help Notice Placeholder --><div class=\"mt-8 p-4 bg-gray-50 border border-gray-200 rounded-lg dark:bg-gray-800 dark:border-gray-700\"><div class=\"flex\"><div class=\"flex-shrink-0\"><i class=\"fas fa-info-circle text-blue-400 dark:text-blue-400\"></i></div><div class=\"ml-3\"><p class=\"text-sm text-blue-700 dark:text-blue-400\">Notification services allow the system to send alerts for job events such as completion, errors, or when jobs start.</p></div></div></div></div></div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = dialog.DialogScripts().Render(ctx, templ_7745c5c3_Buffer)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			return nil
		})
		templ_7745c5c3_Err = components.LayoutWithContext("Notification Services", ctx).Render(templ.WithChildren(ctx, templ_7745c5c3_Var2), templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
