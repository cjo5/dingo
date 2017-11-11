int puts(const char*);

int puti(int i);

int geta();

int getb();


int main() {
        int a = geta();
        int b = getb();
        int c = (a == 2) || (b == 3);

        puti(c);

        /*
        if ((a == 2) && (b == 3)) {
                puts("true");
        }
        */
        return 0;
}