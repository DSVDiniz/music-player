#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

out vec4 finalColor;

uniform float dimFactor;
uniform sampler2D texture0;

void main()
{
    vec4 texColor = texture(texture0, fragTexCoord);
    vec4 color = texColor * fragColor;
    finalColor = vec4(color.rgb * dimFactor, color.a);
}