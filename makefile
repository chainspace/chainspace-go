FILES=	service/types.proto\
	service/transactor/types.proto\
	service/broadcast/types.proto

proto:
	$(foreach f,$(FILES),\
		./genproto.sh $(f);\
	)
