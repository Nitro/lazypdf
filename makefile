MUPDF_VERSION = "1.16.1"

install-mupdf:
	@rm -Rf tmp/mupdf
	@git clone --branch ${MUPDF_VERSION} --depth 1 --recurse-submodules=thirdparty https://github.com/ArtifexSoftware/mupdf.git tmp/mupdf
	@cd tmp/mupdf; make HAVE_X11=no HAVE_GLUT=no install
