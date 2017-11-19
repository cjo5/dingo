int puts(const char* s);

int puti(int i);

int putsh(short i);
int putf(float i);

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

//struct Vec3 globalVec3 = {.v.x = 5, .v.y = 13, .z = 9 };

int main() {
        struct Vec3 localVec3 = {.v.x = 1, .v.y = 3, .z = 2 };
        int a = localVec3.v.y * 3;
        puti(a);
        return 0;
}