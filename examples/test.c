int puts(const char*);

int puti(int i);

int putsh(short i);
int putf(float i);

int getf();

int geta();

int getb();

short getc();

static char* x = "hello";

int main() {
        int a = geta();
        unsigned short b = (unsigned short)a;
        return 0;
}