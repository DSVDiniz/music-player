#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

out vec4 finalColor;

uniform vec2 resolution;
uniform float time;
uniform vec3 uColor;

void main()
{
    vec2 fragCoord = fragTexCoord * resolution;

    float interval = 2*12.0;
    float speed = 40.0;
    float coord = fragCoord.x + fragCoord.y + time * speed;
    float a = step(mod(coord, interval) / (interval - 1.0), 0.5);
    vec3 color = mix(vec3(0.0), uColor, a);

    finalColor = vec4(color, 1.0);
}
