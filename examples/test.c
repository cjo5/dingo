int puts(const char* s);

int puti(int i);

int putsh(short i);
int putf(float i);

int putptr(int*);

int getf();

int geta();

int getb();

short getc();

static char* x = "hello";

struct Vec2 {
        int x;
        int y;
};

struct Vec3 {
        int z;
        struct Vec2 v;
};

void printVec(struct Vec3 v) {
        puti(v.v.x + v.v.y + v.z);
}


struct List {
        struct Vec2* v;
};


//struct Vec3 globalVec3 = {.v.x = 5, .v.y = 13, .z = 9 };

int main() {
        int a[] = {1, 2, 3, 4};
        int b = a[1];
        puti(b);
        
        return 0;
}