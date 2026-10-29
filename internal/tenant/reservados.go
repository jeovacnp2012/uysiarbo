package tenant

// Reservados são subdomínios que não podem ser escolhidos por um cliente: já têm outro uso na VPS
// (outros sistemas em inovarsistemas.biz) ou são nomes de serviço.
var Reservados = map[string]bool{
	"www": true, "mail": true, "ftp": true, "smtp": true, "pop": true, "imap": true, "webmail": true,
	"webdisk": true, "cpanel": true, "cpcalendars": true, "cpcontacts": true, "whm": true,
	"admin": true, "api": true, "app": true, "static": true, "uysiarbo": true,
	"labpront": true, "labcnp": true, "labmutum": true, "perene": true, "lavexpress": true,
	"pitcash": true, "milhaobiblico": true, "digitalpront": true, "gamer": true, "genclube": true,
	"chamados": true,
}
