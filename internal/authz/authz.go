package authz

// Principal é quem está agindo: o master da plataforma ou um usuário de um cliente com as
// permissões do seu papel.
type Principal struct {
	Master bool
	perms  map[string]struct{}
}

// Master devolve o principal master: nenhuma regra de permissão, bloco ou escopo se aplica a ele.
func Master() Principal { return Principal{Master: true} }

// Com devolve um principal comum com as permissões dadas.
func Com(codigos ...string) Principal {
	p := Principal{perms: make(map[string]struct{}, len(codigos))}
	for _, c := range codigos {
		p.perms[c] = struct{}{}
	}
	return p
}

// Pode diz se a ação é permitida. O master pode tudo, inclusive códigos que ainda não existem.
func (p Principal) Pode(codigo string) bool {
	if p.Master {
		return true
	}
	_, ok := p.perms[codigo]
	return ok
}

// PodeAlguma diz se pelo menos uma das ações é permitida (ex.: mostrar um item de menu).
func (p Principal) PodeAlguma(codigos ...string) bool {
	for _, c := range codigos {
		if p.Pode(c) {
			return true
		}
	}
	return false
}

// PodeBloco diz se há ao menos uma permissão do bloco (para mostrar ou esconder o bloco no menu).
func (p Principal) PodeBloco(cat *Catalogo, bloco string) bool {
	if p.Master {
		return true
	}
	for _, b := range cat.Blocos {
		if b.Nome != bloco {
			continue
		}
		for _, g := range b.Grupos {
			for _, perm := range g.Permissoes {
				if !perm.Restritiva && p.Pode(perm.Codigo) {
					return true
				}
			}
		}
	}
	return false
}

// Restrito diz se uma permissão RESTRITIVA vale para a pessoa (ex.: "visita_visualizar_minha" =
// só vê o que é dela). O master nunca é restringido.
func (p Principal) Restrito(codigo string) bool {
	if p.Master {
		return false
	}
	_, ok := p.perms[codigo]
	return ok
}

// Codigos devolve as permissões concedidas (vazio para o master, que não depende delas).
func (p Principal) Codigos() []string {
	out := make([]string, 0, len(p.perms))
	for c := range p.perms {
		out = append(out, c)
	}
	return out
}
