package localizable

const SPANISH = "es"

const ES = `
"appName" = "cryptopower";
"appTitle" = "cryptopower (%s)";
"recentTransactions" = "Transacciones Recientes";
"recentProposals" = "Propuestas Recientes";
"seeAll" = "Ver todo";
"send" = "Enviar";
"receive" = "Recibir";
"online" = "En línea, ";
"offline" = "Desconectado, ";
"viewDetails" = "View details";
"hideDetails" = "Ocultar detalles";
"peers" = "Compañeros";
"connectedPeersCount" = "Número de nodos conectados";
"noConnectedPeer" = "No hay nodos conectados.";
"disconnect" = "Desconectar";
"reconnect" = "Volver a conectar";
"currentTotalBalance" = "Saldo total actual";
"totalBalance" = "Saldo total";
"walletStatus" = "Estado de la cartera";
"blockHeaderFetched" = "Encabezados de bloque obtenidos";
"noTransactions" = "No hay transacciones.";
"transactionDetails" = "Transaction Details";
"ticketDetails" = "Ticket details"
"fetchingBlockHeaders" = "Obteniendo encabezados de bloque · %v%%";
"discoveringWalletAddress" = "Descubriendo la dirección de la billetera · %v%%";
"rescanningHeaders" = "Volver a escanear encabezados · %v%%";
"rescanningBlocks" = "Volver a escanear bloques";
"blocksScanned" = "Bloques escaneados";
"blocksLeft" = "%d quedan bloques";
"sync" = "Sincronización";
"autoSyncInfo" = "La función de sincronización automática se ha habilitado y las billeteras no están sincronizadas.\nLe gustaría comenzar a sincronizar sus billeteras ahora?";
"syncSteps" = "Paso %d/3";
"blockHeaderFetchedCount" = "%d de %d";
"timeLeft" = "%v restante";
"connectedTo" = "Conectado a";
"accRenamed" = "Account renamed"
"connecting" = "Conectando...";
"synced" = "Sincronizado";
"syncingState" = "Sincronizando...";
"waitingState" = "Esperando...";
"walletNotSynced" = "No sincronizado";
"rebroadcast" = "Retransmitir";
"cancel" = "Cancelar";
"resumeAccountDiscoveryTitle" = "Desbloquear para reanudar la restauración";
"unlock" = "Desbloquear";
"unlockWithPassword" = "Desbloquear con contraseña"
"syncingProgress" = "Progreso de la sincronización";
"syncingProgressStat" = "%s detrás";
"noWalletLoaded" = "Sin cartera cargada";
"lastBlockHeight" = "Altura del último bloque";
"ago" = "atrás";
"newest" = "Recientes";
"oldest" = "Antiguas";
"all" = "Todas";
"transferred" = "Transferida";
"sent" = "Enviadas";
"received" = "Recibidas";
"yourself" = "A la misma cuenta";
"mixed" = "Mixte";
"unmined" = "sin minar";
"immature" = "Inmaduro";
"voted" = "Votado";
"revoked" = "Revocado";
"live" = "Vivir";
"expired" = "Caducado";
"purchased" = "Comprado";
"revocation" = "Revocación";
"staking" = "replanteo";
"immatureRewards" = "Recompensas inmaduras";
"lockedByTickets" = "Bloqueado por boletos";
"immatureStakeGen" = "Generación de estaca inmadura";
"votingAuthority" = "Autoridad de votación";
"unknown" = "Desconocido";
"nConfirmations" = "%d Confirmaciones";
"from" = "De";
"to" = "Enviar a";
"fee" = "Tarifa";
"includedInBlock" = "Incluido en bloque";
"type" = "Type";
"transactionId" = "ID de la transacción";
"xInputsConsumed" = "%d Entradas consumidas";
"xOutputCreated" = "%d Salidas creadas";
"viewOnExplorer" = "View on block explorer";
"watchOnlyWallets" = "Carteras de solo mirar";
"signMessage" = "Firmar mensaje";
"verifyMessage" = "Verificar mensaje";
"message" = "Mensaje";
"viewProperty" = "Ver propiedad";
"stakeShuffle" = "StakeShuffle";
"rename" = "Renombrar";
"renameWalletSheetTitle" = "Change wallet name";
"securityToolsInfo" = "%v Various tools that help in different aspects of crypto currency security will be located here. %v"
"settings" = "Configuración";
"createANewWallet" = "Crear una nueva cartera"
"importExistingWallet" = "Importar una cartera existente";
"importWatchingOnlyWallet" = "Importar una cartera de solo mirar";
"create" = "Crear";
"watchOnlyWalletImported" = "Cartera de solo mirar importada";
"addNewAccount" = "Agregar una nueva cuenta";
"notBackedUp" = "Cuenta no respaldada";
"labelSpendable" = "Disponible";
"backupSeedPhrase" = "Copia de seguridad de las palabras semilla";
"verifySeedInfo" = "Verifique la copia de seguridad de sus palabras semilla para que pueda recuperar sus fondos cuando sea necesario.";
"createNewAccount" = "Crear una nueva cuenta";
"invalidPassphrase" = "La contraseña ingresada no era válida.";
"passwordNotMatch" = "Las contraseñas no coinciden"
"Import" = "Importar";
"spendingPasswordInfo" = "Una contraseña de gastos ayuda a proteger las transacciones de su billetera."
"spendingPasswordInfo2" = "Esta contraseña de gasto es solo para la nueva billetera"
"spendingPassword" = "Pasar contraseña";
"enterSpendingPassword" = "Ingrese la contraseña de gastos"
"confirmSpendingPassword" = "Confirmar contraseña de gastos";
"changeSpendingPass" = "Cambiar contraseña de gastos";
"newSpendingPassword" = "Nueva contraseña de gastos";
"confirmNewSpendingPassword" = "Confirmar contraseña de gasto";
"spendingPasswordUpdated" = "Contraseña de gastos actualizada";
"notifications" = "Notificaciones";
"beepForNewBlocks" = "Beep (notificación) para los nuevos bloques";
"debug" = "Depurar";
"rescanBlockchain" = "Volver a escanear la blockchain";
"dangerZone" = "Zona peligrosa";
"removeWallet" = "Eliminar la cartera del dispositivo";
"change" = "Cambiar";
"notConnected" = "No conectado a la red de decred";
"rescanProgressNotification" = "Verifique el progreso en el resumen de la cuenta.";
"rescanInfo" = "Volver a escanear puede ayudar a resolver algunos errores de saldo. Esto llevará algún tiempo, ya que escanea toda la cadena de bloques en busca de transacciones."
"rescan" = "Volver a escanear";
"confirmToRemove" = "Confirmar para eliminar";
"remove" = "Eliminar";
"confirm" = "Confirmar";
"general" = "General";
"unconfirmedFunds" = "Gastar fondos no confirmados";
"confirmed" = "Confirmado";
"exchangeRate" = "Obtener tipo de cambio";
"language" = "Idioma";
"security" = "Seguridad";
"newStartupPass" = "Nueva contraseña de inicio"
"confirmNewStartupPass" = "Confirmar nueva contraseña de inicio"
"startupPassConfirm" = "Contraseña de inicio cambiada"
"startupPasswordEnabled" = "Contraseña de inicio habilitada"
"setupStartupPassword" = "Configurar contraseña de inicio"
"startupPasswordInfo" = "La contraseña de inicio ayuda a proteger su billetera del acceso no autorizado".
"confirmStartupPass" = "Confirmar contraseña de inicio actual"
"currentStartupPass" = "Contraseña de inicio actual"
"startupPassword" = "Contraseña de inicio";
"changeStartupPassword" = "Cambiar la contraseña de inicio";
"connection" = "Conexión";
"connectToSpecificPeer" = "Conectarse a un nodo específico";
"changeSpecificPeer" = "Cambiar un nodo en específico";
"CustomUserAgent" = "Agente de usuario personalizado";
"userAgentSummary" = "Para obtener el tipo de cambio";
"changeUserAgent" = "Cambiar el agente de usuario";
"createStartupPassword" = "Crea una contraseña de inicio";
"confirmRemoveStartupPass" = "Confirmar para desactivar la contraseña de inicio";
"userAgentDialogTitle" = "Configurar el agente de usuario";
"overview" = "Resumen";
"transactions" = "Transacciones";
"wallets" = "Carteras";
"tickets" = "Tickets";
"more" = "Más Opciones";
"english" = "Inglés";
"french" = "Francés";
"spanish" = "Español";
"usdBittrex" = "USD (Bittrex)";
"none" = "Ninguno";
"proposals" = "Propuestas";
"governance" = "Gobernancia";
"staking" = "replanteo";
"pending" = "Pendiente";
"vote" = "Votar";
"revoke" = "Revocar";
"maturity" = "Madurez";
"yesterday" = "el dia de ayer";
"days" = "dias";
"hours" = "Horas";
"mins" = "minutos";
"secs" = "segundos";
"weekAgo" = "%d semana ago";
"weeksAgo" = "%d semanas ago";
"yearAgo" = "%d año ago";
"yearsAgo" = "%d años ago";  
"monthAgo" = "%d mes ago";
"monthsAgo" = "%d meses ago";
"dayAgo" = "%d día ago";
"daysAgo" = "%d dias ago";
"hourAgo" = "%d hora ago";
"hoursAgo" = "%d horas ago";
"minuteAgo" = "%d minuto ago";
"minutesAgo" = "%d minutos ago";
"justNow" = "Justo ahora";
"imawareOfRisk" = "Entiendo los riesgos";
"unmixedBalance" = "Saldo sin mezclar";
"backupLater" = "Copia de seguridad más tarde";
"backupNow" = "Copia ahora";
"status" = "Estado";
"daysToVote" = "Días para votar";
"reward" = "Premio";
"viewTicket" = "Ver ticket asociado";
"external" = "Externo";
"republished" = "Transacciones no minadas republicadas en la red decretada";
"copied" = "Copiado";
"txHashCopied" = "Hash de transacción copiado"";
"addressCopied" = "Dirección copiada";
"address" = "Dirección";
"acctDetailsKey" = "%d externo, %d interno, %d importado";
"key" = "Llave"
"acctNum" = "Número de cuenta";
"acctName" = "Nombre de la cuenta";
"acctRenamed" = "Cuenta renombrada";
"acctCreated" = "Cuenta creada"
"renameAcct" = "Vuelva a nombrar la cuenta";
"hdPath" = "Ruta HD";
"validateWalSeed" = "Validar semillas de billetera";
"clearAll" = "Limpiar todo";
"restoreWallet" = "Restaurar billetera";
"restoreExistingWallet" = "Restaurar una billetera existente";
"enterSeedPhrase" = "Ingrese su frase inicial";
"invalidSeedPhrase" = "Frase inicial no válida"
"seedAlreadyExist" = "Ya existe una billetera con una semilla idéntica."
"walletExist" = "Monedero con nombre: %s ya existe"
"walletNotExist" = "Monedero con ID: %v no existe"
"walletRestored" = "Monedero restaurado"
"enterWalletDetails" = "Ingrese los detalles de la billetera"
"copy" = "Dupdo"
"howToCopy" = "como copiar"
"enterAddressToSign" = "Introduce una dirección y un mensaje para firmar:"
"signCopied" = "Firma copiada"
"signature" = "Firma"
"confirmToSign" = "Confirmar para firmar"
"enterValidAddress" = "Por favor introduce una dirección válida"
"enterValidMsg" = "Por favor ingrese un mensaje válido para firmar"
"invalidAddress" = "Dirección inválida"
"validAddress" = "Dirección válida"
"addrNotOwned" = "Dirección que no pertenece a ningún monedero"
"delete" = "Borrar"
"walletName" = "Nombre de la billetera"
"enterWalletName" = "Ingrese el nombre de la billetera"
"walletRenamed" = "Monedero renombrado"
"walletCreated" = "Monedero creado"
"addWallet" = "Agregar billetera"
"selectWallet" = "Sélectionner le portefeuille"
"checkMixerStatus" = "Comprobar el estado del mezclador"
"walletRestoreMsg" = "Puede restaurar esta billetera desde la palabra inicial después de que se elimine."
"walletRemoved" = "Monedero eliminado"
"walletRemoveInfo" = "Asegúrese de tener una copia de seguridad de la palabra semilla antes de quitar la billetera"
"watchOnlyWalletRemoveInfo" = "La billetera de solo reloj se eliminará de su aplicación"
"gotIt" = "Entiendo"
"noValidAccountFound" = "no se encontró una cuenta válida"
"mixer" = "Mezclador"
"readyToMix" = "Listo para mezclar"
"mixerRunning" = "El mezclador está funcionando..."
"useMixer" = "¿Cómo usar la batidora?"
"keepAppOpen" = "Mantener esta aplicación abierta"
"mixerShutdown" = "La mezcladora se detendrá automáticamente cuando el resto sin mezclar esté completamente mezclado."
"votingPreference" = "Preferencia de voto:"
"noAgendaYet" = "No hay agendas todavía"
"fetchingAgenda" = "Obteniendo agendas"
"updatePreference" = "Preferencia de actualización"
"approved" = "Aprobado"
"voting" = "Votación"
"rejected" = "Rechazado"
"abandoned" = "Abandonado"
"inDiscussion" = "en discusión"
"fetchingProposals" = "Obtener propuestas..."
"fetchProposals" = "Obtener propuestas"
"underReview" = "Bajo revisión"
"noProposal" = "Sin propuestas %s"
"waitingForAuthor" = "Esperando a que el autor autorice la votación"
"waitingForAdmin" = "Esperando que el administrador active el inicio de la votación"
"voteTooltip" = "%d %% Sí se requieren votos para la aprobación"
"yes" = "Sí: "
"no" = "No: "
"totalVotes" = "Total de votos:  %6.0f"
"totalVotesReverse" = "%d Total de votos"
"quorumRequirement" = "Cuyo requerimiento:  %6.0f"
"discussions" = "Discusiones:   %d comentarios"
"published" = "Publicado:   %s"
"token" = "Simbólico:   %s"
"proposalVoteDetails" = "Detalles de la votación de la propuesta"
"votingServiceProvider" = "Proveedor de servicios de votación"
"selectVSP" = "Seleccione VSP..."
"addVSP" = "Agregar un nuevo VSP..."
"save" = "Guardar"
"noVSPLoaded" = "No se ha cargado vsp. Compruebe la conexión a Internet y vuelva a intentarlo."
"extendedPubKey" = "Clave pública extendida"
"enterXpubKey" = "ingrese una clave pública extendida válida"
"xpubKeyErr" = "Error comprobando xpub: %v"
"xpubWalletExist" = "Ya existe una billetera con una clave pública extendida idéntica."
"hint" = "insinuación"
"addAcctWarn" = "%v Las cuentas %v no se pueden %v eliminar una vez creadas.%v"
"verifyMessageInfo" = "%v You can use this form to verify the signature's validity after you or your counterparty have generated one.%v After you've input the address, message, and signature, you'll see VALID if the signature matches the address and message correctly, and INVALID otherwise.%v"
"setupMixerInfo" = "%v Se crearán dos cuentas dedicadas %v mixtas %v y %v no mixtas %v para usar el mezclador. %v Esta acción no se puede deshacer. %v"
"txdetailsInfo" = "%v Toque %v texto azul %v para copiar el elemento %v"
"backupInfo" = "%v No hay copia de seguridad, ¡no hay monedas! %v Para no perder sus monedas cuando su dispositivo se pierde o se rompe, haga una copia de seguridad de la billetera %v ¡Ahora %v y guárdela en %v un lugar seguro! %v"
"signMessageInfo" = "%v Firmar un mensaje con la clave privada de una dirección le permite demostrar que es el propietario de una dirección dada a una posible contraparte. %v"
"allowUnspendUnmixedAcct" = "%v Los gastos de cuentas no mixtas podrían rastrearse hasta usted %v Escriba %v Entiendo los riesgos %v para permitir gastos de cuentas no mixtas.%v"
"balToMaintain" = "Equilibrio a mantener (DCR)"
"autoTicketPurchase" = "compra de billetes de auto"
"purchasingAcct" = "cuenta de compras"
"locked" = "Bloqueado"
"balance" = "Equilibrio"
"allTickets" = "Todas las entradas"
"noTickets" = "Todavía no hay entradas"
"noActiveTickets" = "Sin entradas activas"
"liveTickets" = "Boletos en vivo"
"ticketRecord" = "Registro de entradas"
"rewardsEarned" = "Recompensas ganadas"
"noReward" = "Stakey no ve recompensas"
"loadingPrice" = "Prix de chargement"
 "notAvailable" = "No disponible"
"ticketConfirmed" = "Boleto(s) confirmado(s)"
"backStaking" = "Volver a apostar"
"ticketSettingSaved" = "La configuración de compra automática de boletos se guardó correctamente".
"autoTicketWarn" = "La configuración no se puede modificar cuando se está ejecutando el comprador de boletos".
"autoTicketInfo" = "Cryptopower debe seguir funcionando, para que los boletos se compren automáticamente"
"confirmPurchase" = "Confirmar compra automática de boletos"
"ticketError" = "Error de cuenta de comprador de entradas: %v"
"walletToPurchaseFrom" = "Monedero para comprar desde: %s"
"selectedAcct" = "Cuenta seleccionada: %s"
"balToMaintainValue" = "Saldo a mantener: %2.f"
"stake" = "Apostar"
"total" = "Total"
"insufficentFund" = "Fondos insuficientes"
"ticketPrice" = "Precio del billete"
"unminedInfo" = "Esta estaca está esperando en mempool para ser incluida en un bloque".
"immatureInfo" = "Maduración en %v de %v bloques (%v)".
"liveInfo" = "Esperando ser elegido para votar".
"liveInfoDisc" = "El tiempo promedio de votación es de 28 días, pero puede demorar hasta 142 días".
"liveInfoDiscSub" = "Hay un 0,5% de probabilidad de caducar antes de ser elegido para votar (esta caducidad devuelve el precio original de Stake sin recompensa)".
"votedInfo" = "¡Felicitaciones! Esta estaca ha votado".
"votedInfoDisc" = "El precio de la Apostar + la recompensa se podrá gastar después de %d bloques (~%s)".
"revokeInfo" = "Esta estaca ha sido revocada".
"revokeInfoDisc" = "El precio de la Apostar se podrá gastar después de %d bloques (~%s)".
"expiredInfo" = "Esta participación no ha sido elegida para votar dentro de %d bloques y, por lo tanto, expiró".
"expiredInfoDisc" = "Los boletos vencidos se revocarán para devolverle el precio original de Stake".
"expiredInfoDiscSub" = "Si una participación no se revoca automáticamente, use el botón de revocación".
"liveIn" = "Vivir en"
"spendableIn" = "Prescindible en"
"duration" = "%s (%d/%d bloques)"
"daysToMiss" = "Días para perder"
"stakeAge" = "Edad de la Apostar"
"selectOption" = "Seleccione una de las opciones a continuación para votar"
"updateVotePref" = "Actualizar preferencia de votación"
"voteUpdated" = "Preferencia de voto actualizada con éxito"
"votingWallet" = "Cartera de votación"
"voteConfirm" = "Confirmar para votar"
"voteSent" = "¡Voto enviado con éxito, propuestas refrescantes!"
"numberOfVotes" = "Tienes %d votos"
"notEnoughVotes" = "No tienes suficientes votos"
"search" = "Búsqueda"
"consensusChange" = "Cambios de consenso"
"onChainVote" = "Votación en cadena para actualizar las reglas de consenso de la red Decred".
"offChainVote" = "Votación fuera de la cadena para iniciativas de desarrollo y marketing financiadas por el tesoro de Decred".
"consensusDashboard" = "Panel de control de votos por consenso"
"copyLink" = "Copie y pegue el enlace a continuación en su navegador, para ver el panel de votación por consenso"
"webURL" = "URL Web"
"votingDashboard" = "Panel de votación"
"updated" = "Actualizado"
"viewOnPoliteia" = "Estado de Viena"
"votingInProgress" = "Votación en progreso..."
"version" = "Versión"
"published2" = "Publicado"
"howGovernanceWork" = "¿Cómo funciona la gobernanza?"
"governanceInfo" = "La comunidad de Decred puede participar en discusiones de propuestas para nuevas iniciativas y solicitar fondos para estas iniciativas. Las partes interesadas de Decred pueden votar si estas propuestas deben ser aprobadas y pagadas por el Tesoro de Decred. %v ¿Le gustaría buscar y ver las propuestas?% v"
"proposalInfo" = "Las propuestas y las notificaciones de cortesía se pueden habilitar o deshabilitar desde la página de configuración".
"selectTicket" = "Seleccione un boleto para votar"
"hash" = "Picadillo"
"max" = "MAX"
"noValidWalletFound" = "no se encontró una billetera válida"
"securityTools" = "Herramientas de seguridad"
"about" = "Acerca de"
"help" = "Ayudar"
"darkMode" = "Modo oscuro"
"txNotification" = "Notificación de transacción %s"
"propNotification" = "Notificación de propuesta %s"
"httpReq" = "Para solicitud HTTP"
"enabled" = "activado"
"disable" = "Desactivar"
"disabled" = "Desactivado"
"governanceSettingsInfo" = "¿Está seguro de que desea deshabilitar la gobernanza? Esto borrará todas las propuestas disponibles"
"propFetching" = "Propuestas obteniendo %s. %s"
"checkGovernace" = "Consultar la página de Gobernanza"
"removePeer" = "Eliminar compañero específico"
"removePeerWarn" = "¿Está seguro de que desea continuar con la eliminación del compañero específico?"
"removeUserAgent" = "Eliminar agente de usuario"
"removeUserAgentWarn" = "¿Está seguro de que desea continuar con la eliminación del agente de usuario?"
"ipAddress" = "dirección IP"
"userAgent" = "Agente de usuario"
"validateMsg" = "Validar dirección"
"validate" = "Validar"
"helpInfo" = "Para obtener más información, visite la documentación de Decred".
"documentation" = "Documentación"
"verifyMsgNote" = "Ingrese la dirección, firma y mensaje para verificar:"
"verifyMsgError" = "Error al verificar el mensaje: %v"
"invalidSignature" = "Firma o mensaje no válido"
"validSignature" = "Firma válida"
"emptySign" = "El campo no puede estar vacío. Proporcione una firma válida".
"emptyMsg" = "El campo no puede estar vacío. Proporcione un mensaje firmado válido".
"clear" = "Limpiar"
"validateAddr" = "Validar dirección"
"validateNote" = "Ingrese una dirección para validar:"
"owned" = "De tu propiedad en"
"notOwned" = "No es tuyo"
"buildDate" = "La fecha de construcción"
"network" = "Red"
"license" = "Licencia"
"checkWalletLog" = "Comprobar los registros de la billetera"
"checkStatistics" = "Consultar estadísticas"
"statistics" = "Estadísticas"
"dexStartupErr" = "No se puede iniciar el cliente DEX:% v"
"confirmDexReset" = "Confirmar restablecimiento del cliente DEX"
"dexResetInfo" = "Es posible que deba reiniciar Cryptopower antes de poder usar el DEX nuevamente. ¿Continuar?"
"resetDexClient" = "Restablecer cliente DEX"
"walletLog" = "Registro de billetera"
"build" = "Construir"
"peersConnected" = "Compañeros conectados"
"uptime" = "tiempo de actividad"
"bestBlocks" = "mejor bloque"
"bestBlockTimestamp" = "Mejor marca de tiempo del bloque"
"bestBlockAge" = "Mejor edad de bloque"
"walletDirectory" = "Directorio de datos de la billetera"
"dateSize" = "Datos de la billetera"
"exit" = "Salida"
"loading" = "Cargando..."
"openingWallet" = "Apertura de carteras"
"welcomeNote" = "Bienvenido a Decred Wallet, una billetera móvil segura y de código abierto".
"generateAddress" = "Generar nueva dirección"
"receivingAddress" = "Cuenta receptora"
"yourAddress" = "Su dirección"
"receiveInfo" = "Cada vez que recibe un pago, se genera una nueva dirección para proteger su privacidad".
"dcrReceived" = "Has recibido %s DCR"
"ticektVoted" = "Un ticket acaba de votar\nRecompensa de voto: %s DCR"
"ticketRevoked" = "Se revocó un ticket"
"proposalAddedNotif" = "Se agregó una nueva propuesta Nombre: %s"
"voteStartedNotif" = "Comenzó la votación para la propuesta con Token: %s"
"voteEndedNotif" = "La votación ha finalizado para la propuesta con Token: %s"
"newProposalUpdate" = "Nueva actualización para propuesta con Token: %s"
"walletSyncing" = "La billetera se está sincronizando, por favor espere"
"next" = "Próximo"
"retry" = "Rever"
"estimatedTime" = "Hora prevista"
"estimatedSize" = "Tamaño estimado"
"rate" = "Precio"
"totalCost" = "Coste total"
"balanceAfter" = "Saldo después de enviar"
"sendingAcct" = "Cuenta de envío"
"txEstimateErr" = "Error al estimar la transacción: %v"
"sendInfo" = "Ingrese o escanee la dirección de la billetera de destino e ingrese la cantidad para enviar fondos".
"amount" = "Monto"
"txSent" = "¡Transacción enviada!"
"confirmSend" = "Confirmar para enviar"
"sendingFrom" = "Enviando desde"
"sendWarning" = "Su DCR se enviará después de este paso".
"destAddr" = "Dirección de destino"
"myAcct" = "Mi cuenta"
"selectWalletToOpen" = "Select the wallet you would like to open."
"continue" = "Continuar"
"restore" = "Restaurar"
"newWallet" = "Cartera nueva"
"selectWalletType" = "Seleccione el tipo de billetera que desea crear"
"whatToCallWallet" = "¿Cómo te gustaría llamar a tu billetera?"
"existingWalletName" = "¿Cuál es el nombre de la cartera existente de su billetera?"
syncCompTime" = "Est. Sync completion time"
"info" = "Info"
"changeAccount" = "Cambiar cuenta"
"mixedAccount" = "Cuenta mixta"
"unmixedAccount" = "Unmixed account"
"coordinationServer" = "Servidor de coordinación"
"unmixed" = "sin mezclar"
"allowSpendingFromUnmixedAccount" = "Permitir el gasto de una cuenta no mixta"
"currentSpendingPassword" = "Contraseña de gasto actual"
"ticketRevokedTitle" = "Boleto, revocado"
"ticketVotedTitle" = "Boleto, votado"
"info" = "Info"
"changeWalletName" = "Cambiar el nombre de la billetera"
"account" = "Cuenta"
"selectDexServerToOpen" = "Select the Dex server you would like to open."
"addDexServer" = "Add dex server"
"canBuy" = "Can Buy"
"mix" = "Mezcla"
"cancelMixer" = "¿Cancelar batidora?"
"sureToCancelMixer" = "¿Está seguro de que desea cancelar la acción del mezclador?"
"confirmToMixAcc" = "Confirmar para mezclar cuenta"
"mixerStart" = "El mezclador se inicia con éxito"
"default" = "default"
"setUpPrivacy" = "Using StakeShuffle increases the privacy of your wallet transactions."
"setUpStakeShuffle" = "Set up StakeShuffle"
"ok" = "OK"
"removeWalletInfo" = "%v Are you sure you want to remove %v %s%v? Enter the name of the wallet below to verify. %v"
"propNotif" = "Proposal notification"
"peer" = "Peer"
"confirmUmixedSpending" = "Confirm to allow spending from unmixed accounts"
"ok" = "OK"
"accountMixer" = "AccountMixer"
"copyBlockLink" = "Copy block explorer link"
"lifeSpan" = "Life Span"
"votedOn" = "Voted on"
"missedOn" = "Missed on"
"missedTickets"="Missed Ticket"
"revokeCause" = "Revocation cause"
"expiredOn" = "Expired on"
"purchasedOn" = "Purchased On"
"confStatus" = "Confirmation Status"
"txFee" = "Transaction Fee"
"vsp" = "VSP"
"vspFee" = "VSP Fee"
"walletNameMismatch" = "El nombre de la billetera ingresado no coincide con el seleccionado"
"confirmPending" = "Confirmación requerida"
"multipleMixerAccNeeded" = "Configure el mezclador creando dos cuentas necesarias"
"initiateSetup" = "Iniciar configuración"
"takenAccount" = "Se toma el nombre de la cuenta"
"mixerAccErrorMsg" = "Hay cuentas existentes nombradas mixtas o no mixtas. Por favor, cambie el nombre a otro por ahora. Puede volver a cambiarlos después de la configuración.."
"backAndRename" = "Volver atrás y renombrar"
"moveToUnmixed" = "Mover fondos a una cuenta no mixta"
"seedValidationFailed" = "No se pudo verificar. Revise cada semilla de billetera e intente nuevamente."
"invalidAmount" = "Monto invalido"
`
